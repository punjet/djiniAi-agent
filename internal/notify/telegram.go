package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func SendTelegramMessage(text string) error {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := tgPayload{
		ChatID:    chatID,
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
		return fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return err
	}

	if !tgResp.OK {
		return fmt.Errorf("telegram API returned OK=false: %s (code %d)", tgResp.Description, tgResp.ErrorCode)
	}

	return nil
}

var SendInlineKeyboardFunc = func(text string, keyboard [][]InlineButton) (int64, error) {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return 0, fmt.Errorf("telegram TG_BOT_TOKEN or TG_CHAT_ID missing")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := tgPayload{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
		ReplyMarkup: InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return 0, err
	}

	if !tgResp.OK {
		return 0, fmt.Errorf("telegram API returned OK=false: %s (code %d)", tgResp.Description, tgResp.ErrorCode)
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}

// SendInlineKeyboard sends a message with inline buttons and returns the message ID.
func GetUpdates(offset int64) ([]TGUpdate, error) {
	return GetUpdatesFunc(offset)
}

// SendTelegramMessageID sends a text message and returns its message ID.
func SendTelegramMessageID(text string) (int64, error) {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return 0, nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	payload := tgPayload{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return 0, err
	}

	if !tgResp.OK {
		return 0, fmt.Errorf("telegram API returned OK=false: %s (code %d)", tgResp.Description, tgResp.ErrorCode)
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}

// PinChatMessage pins a message in the configured chat.
