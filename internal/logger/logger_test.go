package logger

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envVal   string
		expected slog.Level
	}{
		{name: "Debug", envVal: "DEBUG", expected: slog.LevelDebug},
		{name: "Info", envVal: "INFO", expected: slog.LevelInfo},
		{name: "Warn", envVal: "WARN", expected: slog.LevelWarn},
		{name: "Error", envVal: "ERROR", expected: slog.LevelError},
		{name: "Unset", envVal: "", expected: slog.LevelInfo},
		{name: "Invalid", envVal: "UNKNOWN", expected: slog.LevelInfo},
		{name: "Lowercase", envVal: "debug", expected: slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv("LOG_LEVEL", tt.envVal)
			} else {
				os.Unsetenv("LOG_LEVEL")
			}
			level := parseLogLevel()
			if level != tt.expected {
				t.Errorf("expected level %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestWithAndFromContext(t *testing.T) {
	t.Run("nil context returns default Log", func(t *testing.T) {
		l := FromContext(nil)
		if l != Log {
			t.Errorf("expected default Log, got %v", l)
		}
	})

	t.Run("empty context returns default Log", func(t *testing.T) {
		ctx := context.Background()
		l := FromContext(ctx)
		if l != Log {
			t.Errorf("expected default Log, got %v", l)
		}
	})

	t.Run("context with stored logger returns stored logger", func(t *testing.T) {
		customLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		ctx := WithContext(context.Background(), customLogger)
		l := FromContext(ctx)
		if l != customLogger {
			t.Errorf("expected custom logger, got %v", l)
		}
	})

	t.Run("WithContext with nil context creates background context", func(t *testing.T) {
		customLogger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		ctx := WithContext(nil, customLogger)
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
		l := FromContext(ctx)
		if l != customLogger {
			t.Errorf("expected custom logger, got %v", l)
		}
	})
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("LOG_LEVEL", "WARN")

	Init(tmpDir)

	if Log == nil {
		t.Fatal("expected initialized Log, got nil")
	}

	logFilePath := filepath.Join(tmpDir, "logs", "djinni-bot.log")
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		t.Errorf("expected log file at %s, but it was not created", logFilePath)
	}
}

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
