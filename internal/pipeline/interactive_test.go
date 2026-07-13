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
	notify.EditMessageTextFunc = func(messageID int64, text string) error {
		return nil
	}
	notify.EditMessageReplyMarkupFunc = func(messageID int64, keyboard [][]notify.InlineButton) error {
		return nil
	}
	notify.AnswerCallbackQueryFunc = func(callbackQueryID string, text string) error {
		return nil
	}

	updatesCount := 0
	notify.GetUpdatesFunc = func(offset int64) ([]notify.TGUpdate, error) {
		updatesCount++
		if updatesCount == 1 {
			return []notify.TGUpdate{
				{
					UpdateID: 100,
					CallbackQuery: &notify.TGCallback{
						ID:   "cb1",
						Data: "apply_edit:job-123",
					},
				},
			}, nil
		}
		if updatesCount == 2 {
			return []notify.TGUpdate{
				{
					UpdateID: 101,
					Message: &notify.TGMessage{
						Text: "Make it more enthusiastic",
					},
				},
			}, nil
		}
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	instruction, accept, err := AskUserForApplyReview(
		ctx,
		bot,
		"Test Co",
		"Developer",
		"http://job.url",
		4.5,
		"cv.pdf",
		"Dear hiring manager...",
		"job-123",
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

	notify.GetUpdatesFunc = func(offset int64) ([]notify.TGUpdate, error) {
		return []notify.TGUpdate{
			{
				UpdateID: 102,
				CallbackQuery: &notify.TGCallback{
					ID:   "cb2",
					Data: "apply_accept:job-123",
				},
			},
		}, nil
	}

	instruction, accept, err = AskUserForApplyReview(
		ctx,
		bot,
		"Test Co",
		"Developer",
		"http://job.url",
		4.5,
		"cv.pdf",
		"Dear hiring manager...",
		"job-123",
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
