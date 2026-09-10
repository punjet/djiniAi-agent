package llm

import "context"

// Provider is the common interface for all LLM backends (Gemini, Ollama, etc.).
// Each implementation receives a system prompt and a user message,
// and returns the generated text or an error.
type Provider interface {
	// GenerateText sends the given system and user messages to the LLM backend
	// and returns the raw text response.
	GenerateText(ctx context.Context, system, user string) (string, error)

	// GenerateEmbedding returns an embedding vector for the given text.
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)

	// Name returns a human-readable label for the provider (e.g. "Gemini (gemini-2.5-flash)").
	Name() string
}
