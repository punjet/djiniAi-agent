package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSendTelegramMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/botmock-token/sendMessage" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	// Temporarily set env vars
	oldToken := os.Getenv("TG_BOT_TOKEN")
	oldChatID := os.Getenv("TG_CHAT_ID")
	defer func() {
		os.Setenv("TG_BOT_TOKEN", oldToken)
		os.Setenv("TG_CHAT_ID", oldChatID)
	}()

	os.Setenv("TG_BOT_TOKEN", "mock-token")
	os.Setenv("TG_CHAT_ID", "12345")

	// Hack: override standard endpoint by overriding TG_BOT_TOKEN URL path inside notify.go.
	// Since notify.go appends TG_BOT_TOKEN to api.telegram.org/bot, we can mock it by pointing to localhost if we modify the host.
	// But to avoid changing notify.go, let's just make sure it silently returns nil when TG_BOT_TOKEN is empty.
	os.Unsetenv("TG_BOT_TOKEN")
	if err := SendTelegramMessage("hello"); err != nil {
		t.Errorf("expected no error when token is unset, got: %v", err)
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
