package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func EditMessageText(messageID int64, text string) error {
	return EditMessageTextFunc(messageID, text)
}

var EditRichMessageTextFunc = func(messageID int64, richMsg InputRichMessage) error {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageText", token)
	payload := tgEditRichPayload{
		ChatID:      chatID,
		MessageID:   messageID,
		RichMessage: richMsg,
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

func EditRichMessageText(messageID int64, richMsg InputRichMessage) error {
	return EditRichMessageTextFunc(messageID, richMsg)
}

var EditMessageReplyMarkupFunc = func(messageID int64, keyboard [][]InlineButton) error {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/editMessageReplyMarkup", token)
	payload := tgEditMarkupPayload{
		ChatID:    chatID,
		MessageID: messageID,
	}
	if keyboard != nil {
		payload.ReplyMarkup = InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		}
	} else {
		// Nil/Empty keyboard removes the markup completely
		payload.ReplyMarkup = InlineKeyboardMarkup{
			InlineKeyboard: [][]InlineButton{},
		}
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

// EditMessageReplyMarkup edits the buttons of a previously sent message.
func EditMessageReplyMarkup(messageID int64, keyboard [][]InlineButton) error {
	return EditMessageReplyMarkupFunc(messageID, keyboard)
}

var AnswerCallbackQueryFunc = func(callbackQueryID string, text string) error {
	token := os.Getenv("TG_BOT_TOKEN")
	if token == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", token)
	payload := tgAnswerPayload{
		CallbackQueryID: callbackQueryID,
		Text:            text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// AnswerCallbackQuery acknowledges a button click event in Telegram.
