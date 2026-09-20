package notify

import (
	"encoding/json"
	"os"
	"testing"
)

func TestSendTelegramMessage(t *testing.T) {
	// Temporarily set env vars
	oldToken := os.Getenv("TG_BOT_TOKEN")
	oldChatID := os.Getenv("TG_CHAT_ID")
	defer func() {
		os.Setenv("TG_BOT_TOKEN", oldToken)
		os.Setenv("TG_CHAT_ID", oldChatID)
	}()

	os.Unsetenv("TG_BOT_TOKEN")
	os.Unsetenv("TG_CHAT_ID")
	if err := SendTelegramMessage("hello"); err == nil {
		t.Errorf("expected error when token/chat_id is unset, got nil")
	}

	if _, err := SendTelegramMessageID("hello"); err == nil {
		t.Errorf("expected error when token/chat_id is unset, got nil")
	}
}

func TestSendRichInlineKeyboard_UnsetEnv(t *testing.T) {
	oldToken := os.Getenv("TG_BOT_TOKEN")
	oldChatID := os.Getenv("TG_CHAT_ID")
	defer func() {
		os.Setenv("TG_BOT_TOKEN", oldToken)
		os.Setenv("TG_CHAT_ID", oldChatID)
	}()

	os.Unsetenv("TG_BOT_TOKEN")
	os.Unsetenv("TG_CHAT_ID")

	_, err := SendRichInlineKeyboard(InputRichMessage{}, nil)
	if err == nil {
		t.Error("expected error when env vars are unset, got nil")
	}
}

func TestInputRichMessage_Serialization(t *testing.T) {
	richMsg := InputRichMessage{
		Blocks: []interface{}{
			InputRichBlockParagraph{
				Type: "paragraph",
				Text: []interface{}{
					RichTextBold{Type: "bold", Text: "Hello"},
					" World",
				},
			},
			InputRichBlockDetails{
				Type:    "details",
				Summary: "Summary text",
				IsOpen:  true,
				Blocks: []interface{}{
					InputRichBlockBlockQuotation{
						Type: "blockquote",
						Blocks: []interface{}{
							InputRichBlockParagraph{
								Type: "paragraph",
								Text: "Quoted text",
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(richMsg)
	if err != nil {
		t.Fatalf("failed to marshal InputRichMessage: %v", err)
	}

	expectedJSON := `{"blocks":[{"type":"paragraph","text":[{"type":"bold","text":"Hello"}," World"]},{"type":"details","summary":"Summary text","blocks":[{"type":"blockquote","blocks":[{"type":"paragraph","text":"Quoted text"}]}],"is_open":true}]}`
	if string(data) != expectedJSON {
		t.Errorf("JSON mismatch.\nExpected: %s\nGot:      %s", expectedJSON, string(data))
	}
}

func TestEditRichMessageText_UnsetEnv(t *testing.T) {
	oldToken := os.Getenv("TG_BOT_TOKEN")
	oldChatID := os.Getenv("TG_CHAT_ID")
	defer func() {
		os.Setenv("TG_BOT_TOKEN", oldToken)
		os.Setenv("TG_CHAT_ID", oldChatID)
	}()

	os.Unsetenv("TG_BOT_TOKEN")
	os.Unsetenv("TG_CHAT_ID")

	err := EditRichMessageText(123, InputRichMessage{})
	if err != nil {
		t.Errorf("expected no error when env vars are unset, got %v", err)
	}
}

func TestTgEditRichPayload_Serialization(t *testing.T) {
	payload := tgEditRichPayload{
		ChatID:    "12345",
		MessageID: 67890,
		RichMessage: InputRichMessage{
			Blocks: []interface{}{
				InputRichBlockParagraph{
					Type: "paragraph",
					Text: "Hello",
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal tgEditRichPayload: %v", err)
	}

	expectedJSON := `{"chat_id":"12345","message_id":67890,"rich_message":{"blocks":[{"type":"paragraph","text":"Hello"}]}}`
	if string(data) != expectedJSON {
		t.Errorf("JSON mismatch.\nExpected: %s\nGot:      %s", expectedJSON, string(data))
	}
}

func TestParseMarkdownToRichMessage(t *testing.T) {
	md := `
# Evaluation Report
**Score:** 4.5
**Archetype:** Senior
**Job ID:** 12345
**URL:** https://djinni.co/jobs/12345

### Block A: Technical Skills
- Go
- Docker

### B) Soft Skills
* Teamwork
`
	msg := ParseMarkdownToRichMessage(md)
	b, _ := json.MarshalIndent(msg, "", "  ")
	t.Log(string(b))
}
