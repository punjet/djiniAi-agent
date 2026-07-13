package pipeline

import (
	"context"
	"fmt"
	"strings"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/notify"
)

// AskUserForApplyReview blocks until the user confirms or rejects the job application in Telegram.
func AskUserForApplyReview(ctx context.Context, bot *notify.TelegramBot, company, role, jobURL string, score float64, cvFileName, coverLetter, jobSlug string) (string, bool, error) {
	text := fmt.Sprintf(
		"📋 *Job Review Required*\n\n"+
			"*Company:* %s\n"+
			"*Role:* %s\n"+
			"*Score:* %.1f/5\n"+
			"*URL:* %s\n"+
			"*CV:* %s\n\n"+
			"*Cover Letter:*\n%s\n\n"+
			"Should I apply to this role?",
		company, role, score, jobURL, cvFileName, coverLetter,
	)

	keyboard := [][]notify.InlineButton{
		{
			{Text: "✅ Submit", CallbackData: "apply_accept:" + jobSlug},
			{Text: "✍️ Edit", CallbackData: "apply_edit:" + jobSlug},
			{Text: "❌ Reject", CallbackData: "apply_reject:" + jobSlug},
		},
	}

	msgID, err := notify.SendInlineKeyboard(text, keyboard)
	if err != nil {
		return "", false, fmt.Errorf("failed to send TG review: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case u := <-bot.UpdateChan:
			if u.CallbackQuery != nil {
				data := u.CallbackQuery.Data
				if strings.HasPrefix(data, "apply_accept:") && strings.HasSuffix(data, jobSlug) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Application Accepted!")
					_ = notify.EditMessageText(msgID, text+"\n\n🟢 *Status:* Application accepted and submitted.")
					_ = notify.EditMessageReplyMarkup(msgID, nil) // remove buttons
					return "", true, nil
				}
				if strings.HasPrefix(data, "apply_reject:") && strings.HasSuffix(data, jobSlug) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Application Rejected!")
					_ = notify.EditMessageText(msgID, text+"\n\n🔴 *Status:* Application rejected (skipped).")
					_ = notify.EditMessageReplyMarkup(msgID, nil) // remove buttons
					return "", false, nil
				}
				if strings.HasPrefix(data, "apply_edit:") && strings.HasSuffix(data, jobSlug) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Waiting for edit instructions...")
					_ = notify.EditMessageText(msgID, text+"\n\n🤖 *Status:* Waiting for you to type what the AI should change in the cover letter...")
					_ = notify.EditMessageReplyMarkup(msgID, nil) // remove buttons

					instruction, err := waitForUserMessage(ctx, bot)
					if err != nil {
						return "", false, err
					}

					_ = notify.EditMessageText(msgID, text+fmt.Sprintf("\n\n🔄 *Status:* Regenerating cover letter using guidance: %q", instruction))
					return "edit:" + instruction, false, nil
				}
			}
		}
	}
}

// AskUserForInboxReview handles interactive approval, custom draft rewriting, and AI instruction loops for auto-replies.
func AskUserForInboxReview(ctx context.Context, bot *notify.TelegramBot, sender, originalMsg, proposedReply string, dialogueID string, threadMsgs []api.ThreadMessage) (string, error) {
	var threadSnippet string
	if len(threadMsgs) > 0 {
		start := len(threadMsgs) - 3
		if start < 0 {
			start = 0
		}
		var snippetBuilder strings.Builder
		snippetBuilder.WriteString("\n📜 *Thread (last messages):*\n")
		for _, msg := range threadMsgs[start:] {
			roleIcon := "🏢"
			if msg.Role == "candidate" {
				roleIcon = "👤"
			}
			snippetBuilder.WriteString(fmt.Sprintf("%s %s: %s\n", roleIcon, msg.Role, msg.Text))
		}
		threadSnippet = snippetBuilder.String()
	}

	text := fmt.Sprintf(
		"✉️ *Recruiter Message Review Required*\n\n"+
			"*From:* %s\n"+
			"*Message:* %q\n"+
			"%s\n"+
			"🤖 *Proposed Reply:* %q",
		sender, originalMsg, threadSnippet, proposedReply,
	)

	keyboard := [][]notify.InlineButton{
		{
			{Text: "✅ Confirm", CallbackData: "inbox_confirm:" + dialogueID},
			{Text: "❌ Reject / Edit", CallbackData: "inbox_reject:" + dialogueID},
		},
	}

	msgID, err := notify.SendInlineKeyboard(text, keyboard)
	if err != nil {
		return "", err
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case u := <-bot.UpdateChan:
			if u.CallbackQuery != nil {
				data := u.CallbackQuery.Data
				if strings.HasPrefix(data, "inbox_confirm:") && strings.HasSuffix(data, dialogueID) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Reply Confirmed!")
					_ = notify.EditMessageText(msgID, text+"\n\n🟢 *Status:* Confirmed and sent.")
					_ = notify.EditMessageReplyMarkup(msgID, nil)
					return proposedReply, nil
				}
				if strings.HasPrefix(data, "inbox_reject:") && strings.HasSuffix(data, dialogueID) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Reply Rejected!")

					// Show the Edit/Explain choices
					editKeyboard := [][]notify.InlineButton{
						{
							{Text: "✍️ Write Manually", CallbackData: "inbox_manual:" + dialogueID},
							{Text: "🤖 Explain to AI", CallbackData: "inbox_explain:" + dialogueID},
						},
					}
					_ = notify.EditMessageReplyMarkup(msgID, editKeyboard)
				}
				if strings.HasPrefix(data, "inbox_manual:") && strings.HasSuffix(data, dialogueID) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Waiting for manual input...")
					_ = notify.EditMessageText(msgID, text+"\n\n✍️ *Status:* Waiting for you to type your manual reply in the chat...")
					_ = notify.EditMessageReplyMarkup(msgID, nil)

					// Loop waiting for a text message from the user
					manualText, err := waitForUserMessage(ctx, bot)
					if err != nil {
						return "", err
					}

					_ = notify.EditMessageText(msgID, text+fmt.Sprintf("\n\n🟢 *Status:* Sent manual reply: %q", manualText))
					return manualText, nil
				}
				if strings.HasPrefix(data, "inbox_explain:") && strings.HasSuffix(data, dialogueID) {
					_ = notify.AnswerCallbackQuery(u.CallbackQuery.ID, "Waiting for explanation...")
					_ = notify.EditMessageText(msgID, text+"\n\n🤖 *Status:* Waiting for you to type what the AI should change...")
					_ = notify.EditMessageReplyMarkup(msgID, nil)

					explanation, err := waitForUserMessage(ctx, bot)
					if err != nil {
						return "", err
					}

					_ = notify.EditMessageText(msgID, text+fmt.Sprintf("\n\n🔄 *Status:* Regenerating reply using guidance: %q", explanation))
					return "explain:" + explanation, nil
				}
			}
		}
	}
}

func waitForUserMessage(ctx context.Context, bot *notify.TelegramBot) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case u := <-bot.UpdateChan:
			if u.Message != nil && u.Message.Text != "" {
				return u.Message.Text, nil
			}
		}
	}
}
