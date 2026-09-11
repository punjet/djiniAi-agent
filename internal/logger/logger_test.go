package logger

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLokiHandler(t *testing.T) {
	var mu sync.Mutex
	var received []lokiPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loki/api/v1/push" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read error: %v", err)
			return
		}
		var payload lokiPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("unmarshal error: %v", err)
			return
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	tempDir := t.TempDir()
	os.Setenv("LOKI_URL", ts.URL)
	defer os.Unsetenv("LOKI_URL")

	InitLogger(tempDir)

	Log.Info("test loki message", "key", "value")

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatalf("expected at least 1 loki payload received")
	}

	payload := received[0]
	if len(payload.Streams) == 0 {
		t.Fatalf("expected stream in payload")
	}
	stream := payload.Streams[0]
	if stream.Stream["app"] != "djini-ai-agent" || stream.Stream["job"] != "djinni-bot" || stream.Stream["environment"] != "production" {
		t.Errorf("unexpected stream labels: %v", stream.Stream)
	}
	if len(stream.Values) == 0 {
		t.Fatalf("expected values in stream")
	}

	logFile := filepath.Join(tempDir, "logs", "djinni-bot.log")
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("expected log file to exist at %s", logFile)
	}
}
