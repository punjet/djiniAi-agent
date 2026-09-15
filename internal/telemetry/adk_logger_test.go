package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"djinni-bot-go/internal/logger"
)

func TestADKSpanProcessor(t *testing.T) {
	tmpDir := t.TempDir()
	logger.InitLogger(tmpDir)

	processor := NewADKSpanProcessor()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(processor))
	tracer := tp.Tracer("test-tracer")

	_, span := tracer.Start(context.Background(), "LLM_Generate")
	span.SetAttributes(
		attribute.String("model", "gemini-2.5-flash"),
		attribute.String("api_key", "AIzaSy_SUPER_SECRET_KEY"),
		attribute.String("prompt", "Hello ADK"),
	)
	span.AddEvent("tool_call", trace.WithAttributes(
		attribute.String("tool_name", "scrape_djinni"),
		attribute.String("sessionid", "secret_session"),
	))
	span.End()

	err := tp.ForceFlush(context.Background())
	require.NoError(t, err)

	logFile := filepath.Join(tmpDir, "logs", "deep_trace.log")
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)

	logStr := string(content)
	
	assert.Contains(t, logStr, "LLM_Generate")
	assert.Contains(t, logStr, "gemini-2.5-flash")
	assert.Contains(t, logStr, "Hello ADK")
	assert.Contains(t, logStr, "tool_call")
	assert.Contains(t, logStr, "scrape_djinni")
	
	assert.Contains(t, logStr, "***MASKED***")
	assert.NotContains(t, logStr, "AIzaSy_SUPER_SECRET_KEY")
	assert.NotContains(t, logStr, "secret_session")
}
