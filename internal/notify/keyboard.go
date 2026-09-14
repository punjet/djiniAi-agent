package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func SendInlineKeyboard(text string, keyboard [][]InlineButton) (int64, error) {
	return SendInlineKeyboardFunc(text, keyboard)
}

var EditMessageTextFunc = func(messageID int64, text string) error {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
	payload := tgEditPayload{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// EditMessageText edits the text of a previously sent message.
func AnswerCallbackQuery(callbackQueryID string, text string) error {
	return AnswerCallbackQueryFunc(callbackQueryID, text)
}

// GetUpdates polls Telegram for any new events/messages.
var GetUpdatesFunc = func(offset int64) ([]TGUpdate, error) {
	token := os.Getenv("TG_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TG_BOT_TOKEN missing")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=10&offset=%d", token, offset)
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var tgResp tgResponse
	if err := json.Unmarshal(bodyBytes, &tgResp); err != nil {
		return nil, fmt.Errorf("failed to parse getUpdates response: %w, body: %s", err, string(bodyBytes))
	}

	if !tgResp.OK {
		return nil, fmt.Errorf("telegram API returned OK=false: %s (code %d)", tgResp.Description, tgResp.ErrorCode)
	}

	var updates []TGUpdate
	if err := json.Unmarshal(tgResp.Result, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

func SendRichInlineKeyboard(richMsg InputRichMessage, keyboard [][]InlineButton) (int64, error) {
	return SendRichInlineKeyboardFunc(richMsg, keyboard)
}
