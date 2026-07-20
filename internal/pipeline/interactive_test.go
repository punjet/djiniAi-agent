package pipeline

import (
	"context"
	"testing"
	"time"

	"djinni-bot-go/internal/notify"
)

func TestApplyReviewEditLoop(t *testing.T) {
	bot := notify.NewTelegramBot()

	notify.SendInlineKeyboardFunc = func(text string, keyboard [][]notify.InlineButton) (int64, error) {
		return 1, nil
	}
	notify.SendRichInlineKeyboardFunc = func(richMsg notify.InputRichMessage, keyboard [][]notify.InlineButton) (int64, error) {
		return 1, nil
	}
	notify.EditMessageTextFunc = func(messageID int64, text string) error {
		return nil
	}
	notify.EditMessageReplyMarkupFunc = func(messageID int64, keyboard [][]notify.InlineButton) error {
		return nil
	}
	notify.AnswerCallbackQueryFunc = func(callbackQueryID string, text string) error {
		return nil
	}

	// Mock the external Telegram API functions to control test flow
	var updatesToSend1 []notify.TGUpdate
	updatesToSend1 = append(updatesToSend1, notify.TGUpdate{
		UpdateID: 100,
		CallbackQuery: &notify.TGCallback{
			ID:   "cb1",
			Data: "apply_edit:job-123",
		},
	})
	updatesToSend1 = append(updatesToSend1, notify.TGUpdate{
		UpdateID: 101,
		Message: &notify.TGMessage{
			Text: "Make it more enthusiastic",
		},
	})

	bot.UpdateChan <- updatesToSend1[0] // CallbackQuery
	bot.UpdateChan <- updatesToSend1[1] // Message

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	instruction, accept, _, err := AskUserForApplyReview(
		ctx,
		bot,
		"Test Co",
		"Developer",
		"http://job.url",
		"Some summary description",
		4.5,
		"cv.pdf",
		"Dear hiring manager...",
		"job-123",
		0,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if accept != false {
		t.Errorf("expected accept to be false for edit action, got true")
	}

	expectedInstr := "edit:Make it more enthusiastic"
	if instruction != expectedInstr {
		t.Errorf("expected instruction %q, got %q", expectedInstr, instruction)
	}

	// Second phase: Recreate bot and re-mock GetUpdatesFunc for the second call to AskUserForApplyReview
	bot = notify.NewTelegramBot() // Recreate bot to reset UpdateChan

	// Reassign mock functions for the new bot instance
	notify.SendInlineKeyboardFunc = func(text string, keyboard [][]notify.InlineButton) (int64, error) {
		return 1, nil
	}
	notify.SendRichInlineKeyboardFunc = func(richMsg notify.InputRichMessage, keyboard [][]notify.InlineButton) (int64, error) {
		return 1, nil
	}
	notify.EditMessageTextFunc = func(messageID int64, text string) error {
		return nil
	}
	notify.EditMessageReplyMarkupFunc = func(messageID int64, keyboard [][]notify.InlineButton) error {
		return nil
	}
	notify.AnswerCallbackQueryFunc = func(callbackQueryID string, text string) error {
		return nil
	}

	var updatesToSend2 []notify.TGUpdate
	updatesToSend2 = append(updatesToSend2, notify.TGUpdate{
		UpdateID: 102,
		CallbackQuery: &notify.TGCallback{
			ID:   "cb2",
			Data: "apply_accept:job-123",
		},
	})

	bot.UpdateChan <- updatesToSend2[0] // CallbackQuery

	// Create a new context for the second call, as the previous one might have timed out or been cancelled.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	instruction, accept, _, err = AskUserForApplyReview(
		ctx2, // Use the new context
		bot,
		"Test Co",
		"Developer",
		"http://job.url",
		"Some summary description",
		4.5,
		"cv.pdf",
		"Dear hiring manager...",
		"job-123",
		0,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if accept != true {
		t.Errorf("expected accept to be true, got false")
	}
	if instruction != "" {
		t.Errorf("expected empty instruction, got %q", instruction)
	}
}