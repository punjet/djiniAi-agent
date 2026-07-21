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
	notify.EditRichMessageTextFunc = func(messageID int64, richMsg notify.InputRichMessage) error {
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
	notify.EditRichMessageTextFunc = func(messageID int64, richMsg notify.InputRichMessage) error {
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

func TestAskUserForApplyReview_RichText(t *testing.T) {
	bot := notify.NewTelegramBot()

	var capturedRichMsg notify.InputRichMessage
	var captureCalled bool

	notify.SendRichInlineKeyboardFunc = func(richMsg notify.InputRichMessage, keyboard [][]notify.InlineButton) (int64, error) {
		capturedRichMsg = richMsg
		captureCalled = true
		return 42, nil
	}
	notify.EditRichMessageTextFunc = func(messageID int64, richMsg notify.InputRichMessage) error {
		return nil
	}
	notify.EditMessageReplyMarkupFunc = func(messageID int64, keyboard [][]notify.InlineButton) error {
		return nil
	}
	notify.AnswerCallbackQueryFunc = func(callbackQueryID string, text string) error {
		return nil
	}

	bot.UpdateChan <- notify.TGUpdate{
		UpdateID: 200,
		CallbackQuery: &notify.TGCallback{
			ID:   "cb_rich",
			Data: "apply_accept:job-rich",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, _, err := AskUserForApplyReview(
		ctx,
		bot,
		"Rich Corp",
		"AI Engineer",
		"https://rich.url",
		"Summary details",
		4.8,
		"resume.pdf",
		"Cover Letter draft",
		"job-rich",
		0,
	)
	if err != nil {
		t.Fatalf("AskUserForApplyReview returned error: %v", err)
	}

	if !captureCalled {
		t.Fatal("expected SendRichInlineKeyboardFunc to be called, but it wasn't")
	}

	if len(capturedRichMsg.Blocks) < 4 {
		t.Fatalf("expected at least 4 blocks in RichMessage, got %d", len(capturedRichMsg.Blocks))
	}

	p1, ok := capturedRichMsg.Blocks[0].(notify.InputRichBlockParagraph)
	if !ok {
		t.Fatalf("expected first block to be InputRichBlockParagraph, got %T", capturedRichMsg.Blocks[0])
	}
	p1Texts, ok := p1.Text.([]interface{})
	if !ok {
		t.Fatalf("expected paragraph text to be a slice of interface{}, got %T", p1.Text)
	}

	var hasBoldTitle, hasBoldCompany, hasBoldRole, hasBoldScore, hasBoldURL, hasBoldCV bool
	for _, textObj := range p1Texts {
		if bold, ok := textObj.(notify.RichTextBold); ok {
			switch bold.Text {
			case "Job Review Required":
				hasBoldTitle = true
			case "Company:":
				hasBoldCompany = true
			case "Role:":
				hasBoldRole = true
			case "Score:":
				hasBoldScore = true
			case "URL:":
				hasBoldURL = true
			case "CV:":
				hasBoldCV = true
			}
		}
	}

	if !hasBoldTitle {
		t.Error("missing bold block: 'Job Review Required'")
	}
	if !hasBoldCompany {
		t.Error("missing bold block: 'Company:'")
	}
	if !hasBoldRole {
		t.Error("missing bold block: 'Role:'")
	}
	if !hasBoldScore {
		t.Error("missing bold block: 'Score:'")
	}
	if !hasBoldURL {
		t.Error("missing bold block: 'URL:'")
	}
	if !hasBoldCV {
		t.Error("missing bold block: 'CV:'")
	}

	evalTitle, ok := capturedRichMsg.Blocks[1].(notify.InputRichBlockParagraph)
	if !ok {
		t.Fatalf("expected second block to be InputRichBlockParagraph, got %T", capturedRichMsg.Blocks[1])
	}
	
	evalQuote, ok := capturedRichMsg.Blocks[2].(notify.InputRichBlockBlockQuotation)
	if !ok {
		t.Fatalf("expected third block to be InputRichBlockBlockQuotation, got %T", capturedRichMsg.Blocks[2])
	}
	
	_ = evalTitle
	_ = evalQuote

	p2, ok := capturedRichMsg.Blocks[3].(notify.InputRichBlockParagraph)
	if !ok {
		t.Fatalf("expected fourth block to be InputRichBlockParagraph, got %T", capturedRichMsg.Blocks[3])
	}
	p2Texts, ok := p2.Text.([]interface{})
	if !ok {
		t.Fatalf("expected fourth block text to be a slice of interface{}, got %T", p2.Text)
	}

	var hasBoldCoverLetter bool
	for _, textObj := range p2Texts {
		if bold, ok := textObj.(notify.RichTextBold); ok {
			if bold.Text == "Cover Letter:" {
				hasBoldCoverLetter = true
			}
		}
	}
	if !hasBoldCoverLetter {
		t.Error("missing bold block: 'Cover Letter:'")
	}
}