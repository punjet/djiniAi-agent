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

// DeepTraceLogger handles deep tracing to a separate file.
// It is nil until InitLogger is called.
var DeepTraceLogger *slog.Logger

// LogDeep writes a deep trace log entry with timestamp and stage
func LogDeep(stage, message string, fields ...any) {
	if DeepTraceLogger != nil {
		DeepTraceLogger.Debug(message,
			slog.String("stage", stage),
			slog.Any("fields", fields),
			slog.String("timestamp", time.Now().Format(time.RFC3339Nano)),
		)
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
	handler slog.Handler
	ch      chan []byte
	url     string
	client  *http.Client
	wg      sync.WaitGroup
}

func newLokiHandler(h slog.Handler, lokiURL string) *lokiHandler {
	u := strings.TrimSuffix(lokiURL, "/") + "/loki/api/v1/push"
	lh := &lokiHandler{
		handler: h,
		ch:      make(chan []byte, 1000),
		url:     u,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
	lh.wg.Add(1)
	go lh.worker()
	return lh
}

func (lh *lokiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return lh.handler.Enabled(ctx, level)
}

func (lh *lokiHandler) Handle(ctx context.Context, r slog.Record) error {
	err := lh.handler.Handle(ctx, r)

	var buf bytes.Buffer
	subHandler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		AddSource:   r.Level == slog.LevelError,
		ReplaceAttr: nil,
	})
	if subErr := subHandler.Handle(ctx, r); subErr == nil {
		tsNs := strconv.FormatInt(r.Time.UnixNano(), 10)
		line := buf.String()
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
	}

	return err
}

func (lh *lokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &lokiHandler{
		handler: lh.handler.WithAttrs(attrs),
		ch:      lh.ch,
		url:     lh.url,
		client:  lh.client,
	}
}

func (lh *lokiHandler) WithGroup(name string) slog.Handler {
	return &lokiHandler{
		handler: lh.handler.WithGroup(name),
		ch:      lh.ch,
		url:     lh.url,
		client:  lh.client,
	}
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

	mainWriter := io.MultiWriter(os.Stdout, logFile)
	var handler slog.Handler = slog.NewJSONHandler(mainWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	if lokiURL := os.Getenv("LOKI_URL"); lokiURL != "" {
		handler = newLokiHandler(handler, lokiURL)
		fmt.Printf("Loki log pusher enabled for %s\n", lokiURL)
	}

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
