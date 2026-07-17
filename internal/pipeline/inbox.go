package pipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/notify"
)

type InboxReplyResult struct {
	ShouldReply       bool   `json:"should_reply"`
	ReplyText         string `json:"reply_text"`
	ConversationState string `json:"conversation_state"`
}

// ProcessInbox scans unread Djinni messages, calls LLM to classify and write replies,
// and sends them if should_reply is true.
// Returns a list of dialogues processed and log lines, or error.
func ProcessInbox(ctx context.Context, bot *notify.TelegramBot, panicStop *atomic.Bool, sigChan <-chan os.Signal, cfg *config.Config, engine llm.Engine, contextDir string, dc *client.DjinniClient, dryRun bool) ([]string, error) {
	dialogues, err := api.GetUnreadMessages(dc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch unread messages: %w", err)
	}

	if len(dialogues) == 0 {
		return []string{"No unread messages in inbox."}, nil
	}

	provider, err := llm.NewProvider(cfg, engine, "inbox")
	if err != nil {
		return nil, err
	}

	var logs []string
	logFile := filepath.Join(contextDir, "data", "inbox.log")

	seenIDs, err := loadSeenIDs(logFile)
	if err != nil {
		logs = append(logs, fmt.Sprintf("⚠️  Failed to load seen IDs: %v", err))
	}

	for _, d := range dialogues {
		if panicStop != nil && panicStop.Load() {
			logs = append(logs, "🛑 PanicStop triggered, breaking Inbox loop.")
			break
		}

		if seenIDs != nil && seenIDs[d.ID] {
			continue
		}

		if sigChan != nil {
			interrupted := false
			select {
			case <-sigChan:
				interrupted = true
			default:
			}
			if interrupted {
				break
			}
		}

		// Fetch full thread history for context
		threadMsgs, err := api.GetThreadMessages(dc, d.ID)
		if err != nil {
			// Graceful fallback — proceed with just the last message
			log.Printf("Warning: could not fetch thread for dialog %s: %v", d.ID, err)
		} else {
			d.Messages = threadMsgs
		}

		// 1. Try to find matching evaluation report for context
		reportContent, _ := findReport(contextDir, d.Sender)

		systemPrompt := `You are Kyrylo Kirov, a professional AI Agent Architect. You received an unread message from a recruiter on Djinni.co. Respond to it professionally and concisely according to the Communications Rules.

Communications Rules:
- Language: Mirror the exact same language as the incoming message (Ukrainian, English, or Russian).
- Contact Info: Share email hello@kirov.dev or LinkedIn https://www.linkedin.com/in/kirilo-kirov. NEVER share the phone number (+380637324924) in automatically generated responses.
- Salary Expectation: Target $2,000–$4,000 / month. Minimum: $1,500 / month. If they ask, quote this target range. If their budget is strictly below $1,500, politely reject the role.
- Call/Scheduling: If they ask for a call, ask for their Calendly link or coordinate in the chat.
- Rejection: Politely reject if not related to AI Agents, AI Automation, or AI Integration, if strictly onsite outside Kyiv, or budget < $1500.

Guidelines:
- Keep the reply direct, concise, and professional (1-3 sentences).
- NO MARKDOWN: Do not use ANY markdown formatting (like **, *, #) anywhere in your response. The output must be pure plain text.
- If the recruiter is just saying hello or sending a job spec without questions, politely thank them and ask for the budget range or spec if missing.

Respond in strict JSON format (no markdown wrappers like ` + "`" + `json or comments) matching this schema exactly:
{
  "should_reply": true,
  "reply_text": "...",
  "conversation_state": "concluded|needs-reply|ambiguous"
}

conversation_state rules:
- "concluded": interview scheduled, offer accepted, explicitly rejected, candidate said goodbye, or no further action needed
- "needs-reply": recruiter is waiting for a response from the candidate
- "ambiguous": unclear — use this when unsure`

		guidance := ""
		for {
			var userPrompt string
		if len(d.Messages) > 0 {
			var threadBuilder strings.Builder
			threadBuilder.WriteString("Full conversation thread:\n")
			for _, msg := range d.Messages {
				threadBuilder.WriteString(fmt.Sprintf("[%s] %s: %s\n", msg.Timestamp, msg.Role, msg.Text))
			}
			threadBuilder.WriteString(fmt.Sprintf("\nRecruiter/sender: %s\nLast message to respond to: %s\n\nJob/Company Evaluation Context:\n%s", d.Sender, d.Message, reportContent))
			userPrompt = threadBuilder.String()
		} else {
			userPrompt = fmt.Sprintf(`Recruiter Info:
Sender: %s
Message: "%s"

Job/Company Evaluation Context:
%s`, d.Sender, d.Message, reportContent)
		}

			if guidance != "" {
				userPrompt += fmt.Sprintf("\n\nUser Guidance (follow this to adjust the response): %q", guidance)
			}

			response, err := provider.GenerateText(ctx, systemPrompt, userPrompt)
			if err != nil {
				logs = append(logs, fmt.Sprintf("⚠️  Failed generating reply for %s (dialogue %s): %v", d.Sender, d.ID, err))
				break
			}

			// Clean markdown wrappers if any
			cleanJSON := response
			if idx := strings.Index(cleanJSON, "{"); idx != -1 {
				cleanJSON = cleanJSON[idx:]
			}
			if idx := strings.LastIndex(cleanJSON, "}"); idx != -1 {
				cleanJSON = cleanJSON[:idx+1]
			}

			var res InboxReplyResult
			if err := json.Unmarshal([]byte(cleanJSON), &res); err != nil {
				logs = append(logs, fmt.Sprintf("⚠️  Failed parsing reply JSON for %s: %v. Raw: %q", d.Sender, err, response))
				break
			}

			res.ReplyText = strings.ReplaceAll(res.ReplyText, "**", "")
			res.ReplyText = strings.ReplaceAll(res.ReplyText, "*", "")

			if res.ConversationState == "concluded" {
				skipMsg := fmt.Sprintf("⏭️ Skipped reply to *%s* (dialog %s) — conversation already concluded.", d.Sender, d.ID)
				log.Printf("Skipping dialog %s (%s): conversation already concluded", d.ID, d.Sender)
				_ = notify.SendTelegramMessage(skipMsg)
				break
			}

			if !res.ShouldReply || res.ReplyText == "" {
				logs = append(logs, fmt.Sprintf("⏭  Skipped replying to %s (no reply needed)", d.Sender))
				break
			}

			// Ask user in Telegram for confirmation, custom edit, or explanation
			actionText, err := AskUserForInboxReview(ctx, bot, d.Sender, d.Message, res.ReplyText, d.ID, d.Messages)
			if err != nil {
				logs = append(logs, fmt.Sprintf("⚠️  Failed Telegram inbox review: %v", err))
				break
			}

			// If user gave explanation guidance, re-run LLM loop
			if strings.HasPrefix(actionText, "explain:") {
				guidance = strings.TrimPrefix(actionText, "explain:")
				continue
			}

			finalReply := actionText
			statusLabel := "SENT"
			if dryRun {
				statusLabel = "DRY-RUN (NOT SENT)"
			} else {
				_, err = api.ReplyToMessage(dc, d.ID, finalReply)
				if err != nil {
					logs = append(logs, fmt.Sprintf("❌  Failed sending reply to %s: %v", d.Sender, err))
					break
				}
			}

			logMsg := fmt.Sprintf("[%s] %s -> Reply: %q", statusLabel, d.Sender, finalReply)
			logs = append(logs, logMsg)

			// Log to inbox.log
			if !dryRun {
				f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err == nil {
					dateStr := time.Now().Format("2006-01-02 15:04:05")
					f.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", dateStr, d.ID, d.Sender, finalReply))
					f.Close()
				}
			}
			break
		}
	}

	return logs, nil
}

func findReport(contextDir, sender string) (string, error) {
	// Simple slugification: e.g. "Google / Recruiter" -> "google"
	company := strings.Split(sender, "/")[0]
	company = strings.TrimSpace(company)
	slug := strings.ToLower(company)
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) < 3 {
		return "", fmt.Errorf("company name too short")
	}

	reportsDir := filepath.Join(contextDir, "reports")
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return "", err
	}

	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Name()), slug) {
			data, err := os.ReadFile(filepath.Join(reportsDir, e.Name()))
			if err == nil {
				return string(data), nil
			}
		}
	}
	return "", fmt.Errorf("report not found")
}

func loadSeenIDs(logFile string) (map[string]bool, error) {
	seen := make(map[string]bool)
	f, err := os.Open(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return seen, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 {
			seen[parts[1]] = true
		}
	}
	return seen, scanner.Err()
}
