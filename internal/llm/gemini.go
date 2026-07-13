package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient implements Provider using the official Google Generative AI SDK.
type GeminiClient struct {
	apiKey          string
	model           string
	temperature     float32
	maxOutputTokens int32
}

// GeminiConfig holds configuration for the Gemini client.
type GeminiConfig struct {
	APIKey          string
	Model           string  // default: "gemini-2.5-flash"
	Temperature     float32 // default: 0.4
	MaxOutputTokens int32   // default: 8192
}

// NewGeminiClient creates a new GeminiClient.
func NewGeminiClient(cfg GeminiConfig) (*GeminiClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required for the Gemini provider")
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.4
	}
	if cfg.MaxOutputTokens == 0 {
		cfg.MaxOutputTokens = 8192
	}
	return &GeminiClient{
		apiKey:          cfg.APIKey,
		model:           cfg.Model,
		temperature:     cfg.Temperature,
		maxOutputTokens: cfg.MaxOutputTokens,
	}, nil
}

// Name implements Provider.
func (g *GeminiClient) Name() string {
	return fmt.Sprintf("Gemini (%s)", g.model)
}

// GenerateText implements Provider.
// It sends the system prompt + user message and returns the raw text.
func (g *GeminiClient) GenerateText(ctx context.Context, system, user string) (string, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(g.apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	m := client.GenerativeModel(g.model)
	m.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(system)},
	}
	temp := g.temperature
	m.GenerationConfig = genai.GenerationConfig{
		Temperature:     &temp,
		MaxOutputTokens: &g.maxOutputTokens,
	}

	resp, err := m.GenerateContent(ctx, genai.Text(user))
	if err != nil {
		// Sanitize API key from error messages before returning
		msg := err.Error()
		msg = strings.ReplaceAll(msg, g.apiKey, "[REDACTED]")
		return "", fmt.Errorf("Gemini API error: %s", msg)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("Gemini returned an empty response")
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if t, ok := part.(genai.Text); ok {
			sb.WriteString(string(t))
		}
	}
	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", fmt.Errorf("Gemini returned an empty text response")
	}
	return result, nil
}
