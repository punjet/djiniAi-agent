package llm

import (
	"context"
	"time"

	"djinni-bot-go/internal/logger"
)

// LoggedProvider wraps an underlying llm.Provider and logs text/embedding generation calls with duration and metadata.
type LoggedProvider struct {
	inner Provider
}

// NewLoggedProvider creates a new LoggedProvider wrapping the given inner Provider.
func NewLoggedProvider(inner Provider) Provider {
	return &LoggedProvider{
		inner: inner,
	}
}

// GenerateText delegates to inner Provider, measures execution duration, and logs deep trace.
func (p *LoggedProvider) GenerateText(ctx context.Context, system, user string) (string, error) {
	start := time.Now()
	resp, err := p.inner.GenerateText(ctx, system, user)
	duration := time.Since(start)

	logger.LogDeep(
		"llm_generate_text",
		"GenerateText call",
		"provider", p.inner.Name(),
		"duration_ms", duration.Milliseconds(),
		"system", system,
		"user", user,
		"response", resp,
		"error", err,
	)

	return resp, err
}

// GenerateEmbedding delegates to inner Provider, measures execution duration, and logs deep trace.
func (p *LoggedProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()
	emb, err := p.inner.GenerateEmbedding(ctx, text)
	duration := time.Since(start)

	logger.LogDeep(
		"llm_generate_embedding",
		"GenerateEmbedding call",
		"provider", p.inner.Name(),
		"duration_ms", duration.Milliseconds(),
		"text_len", len(text),
		"embedding_dim", len(emb),
		"error", err,
	)

	return emb, err
}

// Name returns the underlying provider's name.
func (p *LoggedProvider) Name() string {
	return p.inner.Name()
}
