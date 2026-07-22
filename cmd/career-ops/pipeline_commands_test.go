package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"djinni-bot-go/internal/notify"
)

// getUnexportedField retrieves an unexported field from a struct using unsafe.
func getUnexportedField(field reflect.Value) reflect.Value {
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func TestGetLatestReports(t *testing.T) {
	tempDir := t.TempDir()
	reportsDir := filepath.Join(tempDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		t.Fatalf("failed to create temp reports dir: %v", err)
	}

	// Create dummy report files with em-dash (—)
	reportsData := []struct {
		filename string
		content  string
	}{
		{
			filename: "1-google.md",
			content:  "# Evaluation: Google — Software Engineer\n**Score:** 4.5/5\nEvaluation details for Google",
		},
		{
			filename: "3-meta.md",
			content:  "# Evaluation: Meta — Production Engineer\n**Score:** 3.8/5\nEvaluation details for Meta",
		},
		{
			filename: "2-apple.md",
			content:  "# Evaluation: Apple — Go Engineer\n**Score:** 2.9/5\nEvaluation details for Apple",
		},
		{
			filename: "invalid.md",
			content:  "no evaluation header",
		},
	}

	for _, rd := range reportsData {
		err := os.WriteFile(filepath.Join(reportsDir, rd.filename), []byte(rd.content), 0644)
		if err != nil {
			t.Fatalf("failed to write dummy report %s: %v", rd.filename, err)
		}
	}

	reports := getLatestReports(reportsDir, 5)
	if len(reports) != 3 {
		t.Fatalf("expected 3 valid reports, got %d", len(reports))
	}

	// Verify they are sorted by Number in descending order: 3, 2, 1
	if reports[0].Number != 3 || reports[0].Company != "Meta" || reports[0].Role != "Production Engineer" {
		t.Errorf("unexpected report at index 0: %+v", reports[0])
	}
	if reports[1].Number != 2 || reports[1].Company != "Apple" || reports[1].Role != "Go Engineer" {
		t.Errorf("unexpected report at index 1: %+v", reports[1])
	}
	if reports[2].Number != 1 || reports[2].Company != "Google" || reports[2].Role != "Software Engineer" {
		t.Errorf("unexpected report at index 2: %+v", reports[2])
	}

	// Test limit
	limitedReports := getLatestReports(reportsDir, 2)
	if len(limitedReports) != 2 {
		t.Errorf("expected 2 reports under limit, got %d", len(limitedReports))
	}
}

func TestSetupBotCommands(t *testing.T) {
	tempDir := t.TempDir()
	reportsDir := filepath.Join(tempDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		t.Fatalf("failed to create temp reports dir: %v", err)
	}

	// Set global flagContextDir to our temp directory
	oldContextDir := flagContextDir
	flagContextDir = tempDir
	defer func() { flagContextDir = oldContextDir }()

	// Write mock report file
	reportContent := "# Evaluation: Netflix — Backend Architect\n**Score:** 4.8/5\nNetflix evaluation info"
	err := os.WriteFile(filepath.Join(reportsDir, "5-netflix.md"), []byte(reportContent), 0644)
	if err != nil {
		t.Fatalf("failed to write mock report: %v", err)
	}

	// Initialize bot
	bot := notify.NewTelegramBot()
	setupBotCommands(bot, nil, context.Background())

	// Use reflection/unsafe to extract commands and callbackHandlers
	val := reflect.ValueOf(bot).Elem()

	commandsField := val.FieldByName("commands")
	commandsVal := getUnexportedField(commandsField)
	commands := commandsVal.Interface().(map[string]func(*notify.TGMessage))

	callbackHandlersField := val.FieldByName("callbackHandlers")
	callbackHandlersVal := getUnexportedField(callbackHandlersField)
	callbackHandlers := callbackHandlersVal.Interface().(map[string]func(*notify.TGCallback))

	// 1. Verify /stats command is registered
	statsHandler, ok := commands["/stats"]
	if !ok || statsHandler == nil {
		t.Fatal("expected /stats command handler to be registered")
	}

	// 2. Verify stats_report: callback handler is registered
	reportHandler, ok := callbackHandlers["stats_report:"]
	if !ok || reportHandler == nil {
		t.Fatal("expected stats_report: callback handler to be registered")
	}

	// 3. Test executing /stats command
	var sentInlineText string
	var sentButtons [][]notify.InlineButton
	oldSendInline := notify.SendInlineKeyboardFunc
	notify.SendInlineKeyboardFunc = func(text string, keyboard [][]notify.InlineButton) (int64, error) {
		sentInlineText = text
		sentButtons = keyboard
		return 12345, nil
	}
	defer func() { notify.SendInlineKeyboardFunc = oldSendInline }()

	statsHandler(&notify.TGMessage{
		Chat: notify.TGChat{ID: 123},
		Text: "/stats",
	})

	if sentInlineText != "Here are the latest reports:" {
		t.Errorf("expected inline message 'Here are the latest reports:', got %q", sentInlineText)
	}
	if len(sentButtons) != 1 {
		t.Fatalf("expected 1 button row, got %d", len(sentButtons))
	}
	if len(sentButtons[0]) != 1 {
		t.Fatalf("expected 1 button in row, got %d", len(sentButtons[0]))
	}
	btn := sentButtons[0][0]
	if btn.Text != "Netflix — Backend Architect" {
		t.Errorf("expected button text 'Netflix — Backend Architect', got %q", btn.Text)
	}
	if btn.CallbackData != "stats_report:5-netflix.md" {
		t.Errorf("expected callback data 'stats_report:5-netflix.md', got %q", btn.CallbackData)
	}

	// 4. Test executing stats_report: callback handler
	var answeredID, answeredText string
	var sentTextMessages []string
	oldAnswerCallback := notify.AnswerCallbackQueryFunc
	notify.AnswerCallbackQueryFunc = func(id string, text string) error {
		answeredID = id
		answeredText = text
		return nil
	}
	defer func() { notify.AnswerCallbackQueryFunc = oldAnswerCallback }()

	oldSendMessage := notify.SendMessageFunc
	notify.SendMessageFunc = func(text string) error {
		sentTextMessages = append(sentTextMessages, text)
		return nil
	}
	defer func() { notify.SendMessageFunc = oldSendMessage }()

	var sentRichMessages []notify.InputRichMessage
	oldSendRichMessage := notify.SendRichInlineKeyboardFunc
	notify.SendRichInlineKeyboardFunc = func(richMsg notify.InputRichMessage, keyboard [][]notify.InlineButton) (int64, error) {
		sentRichMessages = append(sentRichMessages, richMsg)
		return 67890, nil
	}
	defer func() { notify.SendRichInlineKeyboardFunc = oldSendRichMessage }()

	reportHandler(&notify.TGCallback{
		ID:   "cb_id_789",
		Data: "stats_report:5-netflix.md",
	})

	if answeredID != "cb_id_789" || answeredText != "Loading report..." {
		t.Errorf("unexpected AnswerCallbackQuery call: id=%q, text=%q", answeredID, answeredText)
	}

	if len(sentRichMessages) == 0 {
		t.Errorf("expected at least one Rich message to be sent")
	} else {
		if len(sentRichMessages[0].Blocks) == 0 {
			t.Errorf("expected rich message blocks, got none")
		}
	}
}
