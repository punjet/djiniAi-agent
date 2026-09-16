package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Log is the main structured logger. Initialized with a stdout handler by default
// so it is never nil — even if InitLogger has not been called yet.
var Log *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

type contextKey int

const loggerKey contextKey = iota

// WithContext stores *slog.Logger in context using a private key type.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves *slog.Logger from context.
// If context is nil or does not contain a logger, return default Log.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return Log
	}
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return Log
}

// parseLogLevel parses LOG_LEVEL env var ("DEBUG", "INFO", "WARN", "ERROR").
// Defaults to INFO if unset or invalid.
func parseLogLevel() slog.Level {
	envLevel := strings.ToUpper(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	switch envLevel {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// DeepTraceLogger handles deep tracing to a separate file.
// It is nil until InitLogger is called.
var DeepTraceLogger *slog.Logger

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// LogDeep writes a deep trace log entry with timestamp and stage
func LogDeep(stage, message string, args ...any) {
	if DeepTraceLogger != nil {
		msg := fmt.Sprintf("[%s] %s", stage, message)
		logArgs := append([]any{
			slog.String("stage", stage),
			slog.String("timestamp", time.Now().Format(time.RFC3339Nano)),
		}, args...)
		DeepTraceLogger.Debug(msg, logArgs...)
	}
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type lokiPayload struct {
	Streams []lokiStream `json:"streams"`
}

type lokiHandler struct {
	ch     chan []byte
	url    string
	client *http.Client
	wg     sync.WaitGroup
}

func newLokiHandler(lokiURL string) *lokiHandler {
	u := strings.TrimSuffix(lokiURL, "/") + "/loki/api/v1/push"
	lh := &lokiHandler{
		ch:     make(chan []byte, 1000),
		url:    u,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	lh.wg.Add(1)
	go lh.worker()
	return lh
}

func (lh *lokiHandler) Write(p []byte) (int, error) {
	tsNs := strconv.FormatInt(time.Now().UnixNano(), 10)
	line := strings.TrimSpace(string(p))

	payload, jsonErr := json.Marshal(lokiPayload{
		Streams: []lokiStream{
			{
				Stream: map[string]string{
					"app":         "djini-ai-agent",
					"job":         "djinni-bot",
					"environment": "production",
				},
				Values: [][]string{
					{tsNs, line},
				},
			},
		},
	})
	if jsonErr == nil {
		select {
		case lh.ch <- payload:
		default:
		}
	}

	return len(p), nil
}

func (lh *lokiHandler) worker() {
	defer lh.wg.Done()
	for data := range lh.ch {
		req, err := http.NewRequest("POST", lh.url, bytes.NewReader(data))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := lh.client.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func Init(contextDir string) {
	InitLogger(contextDir)
}

func InitLogger(contextDir string) {
	logDir := filepath.Join(contextDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		Log.Error("Error creating log directory", "error", err)
		os.Exit(1)
	}

	logFile, err := os.OpenFile(filepath.Join(logDir, "djinni-bot.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		Log.Error("Error opening main log file", "error", err)
		os.Exit(1)
	}

	var writers []io.Writer
	writers = append(writers, os.Stdout, logFile)

	if lokiURL := os.Getenv("LOKI_URL"); lokiURL != "" {
		lh := newLokiHandler(lokiURL)
		writers = append(writers, lh)
		fmt.Printf("Loki log pusher enabled for %s\n", lokiURL)
	}

	level := parseLogLevel()
	mainWriter := io.MultiWriter(writers...)
	var handler slog.Handler = slog.NewJSONHandler(mainWriter, &slog.HandlerOptions{
		Level: level,
	})

	Log = slog.New(handler)
	slog.SetDefault(Log)

	deepTraceFilePath := filepath.Join(logDir, "deep_trace.log")
	deepTraceFile, err := os.OpenFile(deepTraceFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		Log.Error("Error opening deep trace log file", "error", err)
		os.Exit(1)
	}
	deepTraceWriter := io.MultiWriter(os.Stderr, deepTraceFile)
	deepTraceFileHandler := slog.NewJSONHandler(deepTraceWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	DeepTraceLogger = slog.New(deepTraceFileHandler)
}
