package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
)

func TestGetUnreadMessages(t *testing.T) {
	mockHtml := `
		Some header content.
		<div data-id="12345" class="proposal-item proposal">
			<div class="company_name">
				<a href="/jobs/company-foo/">Google</a>
				<span> · </span>
				Recruiter Name
			</div>
			<div class="message-text">
				<a class="message-text-inner" href="/my/inbox/12345/">
					Hello, we are interested in your profile.
				</a>
			</div>
		</div>
		<div data-id="67890" class="proposal list-item">
			<div class="header-mobile">
				Meta / Recruiter Meta
			</div>
			<div class="message-text">
				<a class="message-text-inner" href="/my/inbox/67890/">
					Are you available for a call?
				</a>
			</div>
		</div>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my/inbox/" && r.URL.Query().Get("bucket") == "unread" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHtml))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		SessionID: "mock-session",
		CSRFToken: "mock-csrf",
	}
	dc := client.NewDjinniClient(cfg)
	dc.Client.SetBaseURL(server.URL)

	dialogues, err := GetUnreadMessages(dc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(dialogues) != 2 {
		t.Fatalf("expected 2 dialogues, got %d", len(dialogues))
	}

	if dialogues[0].ID != "12345" {
		t.Errorf("expected dialogue 0 ID to be 12345, got %s", dialogues[0].ID)
	}
	if dialogues[0].Sender != "Google / Recruiter Name" {
		t.Errorf("expected dialogue 0 sender to be 'Google / Recruiter Name', got %q", dialogues[0].Sender)
	}
	if dialogues[0].Message != "Hello, we are interested in your profile." {
		t.Errorf("expected dialogue 0 message match, got %q", dialogues[0].Message)
	}

	if dialogues[1].ID != "67890" {
		t.Errorf("expected dialogue 1 ID to be 67890, got %s", dialogues[1].ID)
	}
	if dialogues[1].Sender != "Meta / Recruiter Meta" {
		t.Errorf("expected dialogue 1 sender to be 'Meta / Recruiter Meta', got %q", dialogues[1].Sender)
	}
	if dialogues[1].Message != "Are you available for a call?" {
		t.Errorf("expected dialogue 1 message match, got %q", dialogues[1].Message)
	}
}

func TestReplyToMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my/inbox/12345/" && r.Method == http.MethodPost {
			err := r.ParseForm()
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			msg := r.Form.Get("message")
			csrf := r.Form.Get("csrfmiddlewaretoken")
			if msg == "hello back" && csrf == "mock-csrf" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Success response"))
				return
			}
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := &config.Config{
		SessionID: "mock-session",
		CSRFToken: "mock-csrf",
	}
	dc := client.NewDjinniClient(cfg)
	dc.Client.SetBaseURL(server.URL)

	resp, err := ReplyToMessage(dc, "12345", "hello back")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(resp, "Success response") {
		t.Errorf("expected output to contain 'Success response', got %q", resp)
	}
}

func TestGetThreadMessages(t *testing.T) {
	content, err := os.ReadFile("testdata/thread_page.html")
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	msgs, err := parseThreadMessages(strings.NewReader(string(content)))
	if err != nil {
		t.Fatalf("parseThreadMessages failed: %v", err)
	}

	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(msgs))
	}

	for i, m := range msgs {
		if m.Role == "" {
			t.Fatalf("message %d has empty Role", i)
		}
	}

	lastMsg := msgs[len(msgs)-1]
	if !strings.Contains(strings.ToLower(lastMsg.Text), strings.ToLower("Interview scheduled")) {
		t.Fatalf("expected last message text to contain 'Interview scheduled', got: %q", lastMsg.Text)
	}
}
