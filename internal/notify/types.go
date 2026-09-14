package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	UpdateID      int64       `json:"update_id"`
	Message       *TGMessage  `json:"message"`
	CallbackQuery *TGCallback `json:"callback_query"`
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
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

type tgSendMessageResult struct {
	MessageID int64 `json:"message_id"`
}

// SendTelegramMessage sends a text message to the configured Telegram chat.
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
		return 0, fmt.Errorf("telegram API returned OK=false: %s (code %d)", tgResp.Description, tgResp.ErrorCode)
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}
