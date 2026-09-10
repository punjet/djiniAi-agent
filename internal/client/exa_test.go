package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/imroc/req/v3"

	"djinni-bot-go/internal/config"
)

func TestExaClientSearch(t *testing.T) {
	// Start mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("Unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Fatalf("Unexpected method: %s", r.Method)
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-key" {
			t.Fatalf("Unexpected auth header: %s", authHeader)
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if reqBody["query"] != "test query" {
			t.Fatalf("Unexpected query: %v", reqBody["query"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"markdown": "# Test Result\nThis is a test."}`))
	}))
	defer server.Close()

	// Initialize client with test server URL
	c := req.NewClient()
	c.SetCommonHeader("Authorization", "Bearer test-key")
	c.SetBaseURL(server.URL)

	exaClient := &ExaClient{
		Client: c,
		Config: &config.Config{
			ExaAPIKey: "test-key",
		},
	}

	// Execute search
	markdown, err := exaClient.Search("test query")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	expected := "# Test Result\nThis is a test."
	if markdown != expected {
		t.Errorf("Expected %q, got %q", expected, markdown)
	}
}