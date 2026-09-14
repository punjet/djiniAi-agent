package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/notify"
	"djinni-bot-go/internal/pipeline"

	"github.com/joho/godotenv"
)

type ReportInfo struct {
	Path    string
	Number  int
	Company string
	Role    string
	Date    time.Time
}

func getLatestReports(dir string, limit int) []ReportInfo {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var reports []ReportInfo
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		name := f.Name()
		parts := strings.SplitN(name, "-", 2)
		if len(parts) < 2 {
			continue
		}
		num, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		lines := strings.Split(string(content), "\n")
		company, role := "Unknown", "Unknown"
		for _, line := range lines {
			if strings.HasPrefix(line, "# Evaluation:") {
				header := strings.TrimSpace(strings.TrimPrefix(line, "# Evaluation:"))
				hParts := strings.SplitN(header, "—", 2)
				if len(hParts) == 2 {
					company = strings.TrimSpace(hParts[0])
					role = strings.TrimSpace(hParts[1])
				} else {
					company = header
				}
				break
			}
		}

		info, _ := f.Info()
		reports = append(reports, ReportInfo{
			Path:    filepath.Join(dir, name),
			Number:  num,
			Company: company,
			Role:    role,
			Date:    info.ModTime(),
		})
	}

	sort.Slice(reports, func(i, j int) bool {
		return reports[i].Number > reports[j].Number
	})

	if len(reports) > limit {
		reports = reports[:limit]
	}

	return reports
}

func setupBotCommands(bot *notify.TelegramBot, dc *client.DjinniClient, ctx context.Context) {
	bot.AddCommand("/stats", func(m *notify.TGMessage) {
		reportsDir := filepath.Join(flagContextDir, "reports")
		reports := getLatestReports(reportsDir, 5)
		if len(reports) == 0 {
			notify.SendMessageFunc("No reports found.")
			return
		}

		var keyboard [][]notify.InlineButton
		for _, r := range reports {
			btn := notify.InlineButton{
				Text:         fmt.Sprintf("%s — %s", r.Company, r.Role),
				CallbackData: fmt.Sprintf("stats_report:%s", filepath.Base(r.Path)),
			}
			keyboard = append(keyboard, []notify.InlineButton{btn})
		}

		_, err := notify.SendInlineKeyboard("Here are the latest reports:", keyboard)
		if err != nil {
			notify.SendMessageFunc(fmt.Sprintf("Failed to send stats: %v", err))
		}
	})

	bot.AddCallbackHandler("stats_report:", func(cb *notify.TGCallback) {
		notify.AnswerCallbackQuery(cb.ID, "Loading report...")

		filename := strings.TrimPrefix(cb.Data, "stats_report:")
		reportPath := filepath.Join(flagContextDir, "reports", filename)

		content, err := os.ReadFile(reportPath)
		if err != nil {
			notify.SendMessageFunc(fmt.Sprintf("Could not load report %s: %v", filename, err))
			return
		}

		reportText := string(content)

		if len(reportText) > 4000 {
			reportText = reportText[:4000] + "...\n(truncated)"
		}
		richMsg := notify.ParseMarkdownToRichMessage(reportText)
		_, _ = notify.SendRichInlineKeyboard(richMsg, nil)

		scoreVal := 0.0
		if scoreMatch := regexp.MustCompile(`\*\*Score:\*\*\s*([\d\.]+)/`).FindStringSubmatch(reportText); len(scoreMatch) > 1 {
			if s, err := strconv.ParseFloat(scoreMatch[1], 64); err == nil {
				scoreVal = s
			}
		}

		companySlug := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(filename, ""))

		if scoreVal >= 4.2 {
			outDir := filepath.Join(flagContextDir, "output")
			entries, _ := os.ReadDir(outDir)

			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".pdf") {
					pdfCompanySlug := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(e.Name(), ""))
					if strings.Contains(pdfCompanySlug, companySlug) || strings.Contains(companySlug, pdfCompanySlug) {
						pdfData, _ := os.ReadFile(filepath.Join(outDir, e.Name()))
						if pdfData != nil {
							notify.SendMessageFunc(fmt.Sprintf("Sending generated CV PDF: %s", e.Name()))
							notify.SendDocument(e.Name(), pdfData, "Generated CV")
						}
						break
					}
				}
			}
		}

		appliedMap, err := pipeline.LoadAppliedJobs()
		if err == nil {
			found := false

			var jobID string
			if match := regexp.MustCompile(`(?m)^\*\*Job ID:\*\*\s*(.+)$`).FindStringSubmatch(reportText); match != nil {
				jobID = strings.TrimSpace(match[1])
			}

			if jobID != "" {
				if _, ok := appliedMap[jobID]; ok {
					found = true
				}
			}

			if !found {
				for slug := range appliedMap {
					if strings.Contains(strings.ToLower(slug), strings.ToLower(filename[:len(filename)-3])) {
						found = true
						break
					}
				}
			}

			if found {
				notify.SendMessageFunc("Status: Applied ")
			} else {
				notify.SendMessageFunc("Status: Not Applied (or declined/skipped)")
			}
		}
	})

	bot.AddCommand("/set_session", func(m *notify.TGMessage) {
		parts := strings.SplitN(m.Text, " ", 2)
		if len(parts) < 2 {
			notify.SendTelegramMessage("Usage: `/set_session <new_sessionid>`")
			return
		}
		newToken := strings.TrimSpace(parts[1])

		err := config.UpdateEnvFile(flagContextDir, "DJINNI_SESSIONID", newToken)
		if err != nil {
			notify.SendTelegramMessage(fmt.Sprintf("Failed to update token: %v", err))
			return
		}

		godotenv.Overload(filepath.Join(flagContextDir, ".env"))
		cfg, err := config.LoadConfig()
		if err != nil {
			notify.SendTelegramMessage(fmt.Sprintf("Failed to reload config: %v", err))
			return
		}

		dc.Config = cfg
		dc.Client.SetCommonCookies(nil)

		newDc := client.NewDjinniClient(cfg)
		dc.Client = newDc.Client

		notify.SendTelegramMessage(" Session ID updated successfully. Validating...")

		if api.CheckToken(dc) {
			notify.SendTelegramMessage(" Session ID is valid! Retrying pending applications...")
			go retryPendingApplications(ctx, dc)
		} else {
			notify.SendTelegramMessage(" The new token appears to be invalid or expired. Please check and try again.")
		}
	})
}
