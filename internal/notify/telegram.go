package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type tgPayload struct {
	ChatID      string      `json:"chat_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode,omitempty"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

type tgEditPayload struct {
	ChatID      string      `json:"chat_id"`
	MessageID   int64       `json:"message_id"`
	Text        string      `json:"text"`
	ParseMode   string      `json:"parse_mode,omitempty"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

type tgEditRichPayload struct {
	ChatID      string           `json:"chat_id"`
	MessageID   int64            `json:"message_id"`
	RichMessage InputRichMessage `json:"rich_message"`
	ReplyMarkup interface{}      `json:"reply_markup,omitempty"`
}

type tgEditMarkupPayload struct {
	ChatID      string      `json:"chat_id"`
	MessageID   int64       `json:"message_id"`
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

type tgAnswerPayload struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
}

type InlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineButton `json:"inline_keyboard"`
}

type TGUpdate struct {
	UpdateID      int64          `json:"update_id"`
	Message       *TGMessage     `json:"message"`
	CallbackQuery *TGCallback    `json:"callback_query"`
}

type TGMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      TGChat `json:"chat"`
	Text      string `json:"text"`
}

type TGChat struct {
	ID int64 `json:"id"`
}

type TGCallback struct {
	ID      string     `json:"id"`
	Message *TGMessage `json:"message"`
	Data    string     `json:"data"`
}

type tgResponse struct {
	OK     bool          `json:"ok"`
	Result json.RawMessage `json:"result"`
}

type tgSendMessageResult struct {
	MessageID int64 `json:"message_id"`
}

// SendTelegramMessage sends a text message to the configured Telegram chat.
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
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
		return 0, fmt.Errorf("telegram API returned OK=false: %s", string(tgResp.Result))
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}

// SendInlineKeyboard sends a message with inline buttons and returns the message ID.
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
	client := &http.Client{Timeout: 15 * time.Second}
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
		return nil, fmt.Errorf("telegram getUpdates failed: %s", string(bodyBytes))
	}

	var updates []TGUpdate
	if err := json.Unmarshal(tgResp.Result, &updates); err != nil {
		return nil, err
	}

	return updates, nil
}

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

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	var tgResp tgResponse
	if err := json.NewDecoder(resp.Body).Decode(&tgResp); err != nil {
		return 0, err
	}

	if !tgResp.OK {
		return 0, fmt.Errorf("telegram API returned OK=false: %s", string(tgResp.Result))
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}

// PinChatMessage pins a message in the configured chat.
func PinChatMessage(messageID int64) error {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/pinChatMessage", token)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"disable_notification": true,
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

// SendDocument sends a file to the configured Telegram chat.
func SendDocument(filename string, fileData []byte, caption string) (int64, error) {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return 0, fmt.Errorf("telegram TG_BOT_TOKEN or TG_CHAT_ID missing")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", token)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add chat_id
	_ = writer.WriteField("chat_id", chatID)
	// Add caption
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}

	// Add document
	part, err := writer.CreateFormFile("document", filepath.Base(filename))
	if err != nil {
		return 0, err
	}
	_, err = part.Write(fileData)
	if err != nil {
		return 0, err
	}
	err = writer.Close()
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("telegram document request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var tgResp tgResponse
	if err := json.Unmarshal(respBytes, &tgResp); err != nil {
		return 0, fmt.Errorf("failed to parse sendDocument response: %w, body: %s", err, string(respBytes))
	}

	if !tgResp.OK {
		return 0, fmt.Errorf("telegram sendDocument API returned OK=false: %s", string(tgResp.Result))
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}

type InputRichMessage struct {
	Blocks []interface{} `json:"blocks,omitempty"`
}

type InputRichBlockParagraph struct {
	Type string      `json:"type"`
	Text interface{} `json:"text"`
}

type RichTextBold struct {
	Type string      `json:"type"` // "bold"
	Text interface{} `json:"text"`
}

type InputRichBlockDetails struct {
	Type    string        `json:"type"`
	Summary interface{}   `json:"summary"`
	Blocks  []interface{} `json:"blocks"`
	IsOpen  bool          `json:"is_open,omitempty"`
}

type InputRichBlockBlockQuotation struct {
	Type   string        `json:"type"`
	Blocks []interface{} `json:"blocks"`
	Credit interface{}   `json:"credit,omitempty"`
}

type tgSendRichMessagePayload struct {
	ChatID      string           `json:"chat_id"`
	RichMessage InputRichMessage `json:"rich_message"`
	ReplyMarkup interface{}      `json:"reply_markup,omitempty"`
}

var SendRichInlineKeyboardFunc = func(richMsg InputRichMessage, keyboard [][]InlineButton) (int64, error) {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return 0, fmt.Errorf("telegram TG_BOT_TOKEN or TG_CHAT_ID missing")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendRichMessage", token)
	payload := tgSendRichMessagePayload{
		ChatID:      chatID,
		RichMessage: richMsg,
	}
	if len(keyboard) > 0 {
		payload.ReplyMarkup = InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		}
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
		return 0, fmt.Errorf("telegram API returned OK=false: %s", string(tgResp.Result))
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}

func SendRichInlineKeyboard(richMsg InputRichMessage, keyboard [][]InlineButton) (int64, error) {
	return SendRichInlineKeyboardFunc(richMsg, keyboard)
}
