package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"djinni-bot-go/internal/config"
)

func TestNewDjinniClient(t *testing.T) {
	cfg := &config.Config{
		SessionID: "test-session-id",
		CSRFToken: "test-csrf-token",
	}

	dc := NewDjinniClient(cfg)
	if dc == nil {
		t.Fatal("expected non-nil DjinniClient")
	}

	if dc.Client == nil {
		t.Fatal("expected non-nil req.Client")
	}

	// Verify local config reference
	if dc.Config.SessionID != "test-session-id" || dc.Config.CSRFToken != "test-csrf-token" {
		t.Errorf("Config values mismatch: %+v", dc.Config)
	}

	// Create test server to verify headers and cookies are sent correctly
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Headers
		if r.Header.Get("Referer") != "https://djinni.co" {
			t.Errorf("expected Referer header: https://djinni.co, got: %s", r.Header.Get("Referer"))
		}
		if r.Header.Get("Origin") != "https://djinni.co" {
			t.Errorf("expected Origin header: https://djinni.co, got: %s", r.Header.Get("Origin"))
		}
		if r.Header.Get("X-Csrftoken") != "test-csrf-token" {
			t.Errorf("expected X-Csrftoken header: test-csrf-token, got: %s", r.Header.Get("X-Csrftoken"))
		}

		// Verify Cookies
		sessCookie, err := r.Cookie("sessionid")
		if err != nil {
			t.Errorf("expected sessionid cookie, got error: %v", err)
		} else if sessCookie.Value != "test-session-id" {
			t.Errorf("expected sessionid value: test-session-id, got: %s", sessCookie.Value)
		}

		csrfCookie, err := r.Cookie("csrftoken")
		if err != nil {
			t.Errorf("expected csrftoken cookie, got error: %v", err)
		} else if csrfCookie.Value != "test-csrf-token" {
			t.Errorf("expected csrftoken value: test-csrf-token, got: %s", csrfCookie.Value)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Make request to mock server
	resp, err := dc.Client.R().Get(server.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got: %d", resp.StatusCode)
	}
}
