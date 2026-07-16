package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

var Log *slog.Logger

// DeepTraceLogger handles deep tracing to a separate file
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

func InitLogger(contextDir string) {
	logDir := filepath.Join(contextDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		fmt.Printf("Error creating log directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize main logger — writes to both file and stdout
	logFile, err := os.OpenFile(filepath.Join(logDir, "djinni-bot.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("Error opening main log file: %v\n", err)
		os.Exit(1)
	}

	// MultiWriter: file + stdout so `docker logs` works
	mainWriter := io.MultiWriter(os.Stdout, logFile)
	handler := slog.NewJSONHandler(mainWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	Log = slog.New(handler)
	slog.SetDefault(Log)

	// Initialize deep trace logger — writes to both file and stderr
	deepTraceFilePath := filepath.Join(logDir, "deep_trace.log")
	deepTraceFile, err := os.OpenFile(deepTraceFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Printf("Error opening deep trace log file: %v\n", err)
		os.Exit(1)
	}
	deepTraceWriter := io.MultiWriter(os.Stderr, deepTraceFile)
	deepTraceFileHandler := slog.NewJSONHandler(deepTraceWriter, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	DeepTraceLogger = slog.New(deepTraceFileHandler)
}
