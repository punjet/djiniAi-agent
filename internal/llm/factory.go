package llm

import (
	"context"
	"fmt"
	"os"
	"time"

	"djinni-bot-go/internal/config"
)

var GlobalTraceLogger func(string, ...interface{})

type TraceProvider struct {
	inner Provider
}

func (t *TraceProvider) GenerateText(ctx context.Context, system, user string) (string, error) {
	if GlobalTraceLogger != nil {
		GlobalTraceLogger("--- LLM Request ---")
		GlobalTraceLogger("Provider: %s", t.inner.Name())
		GlobalTraceLogger("System Prompt (len=%d):\n%s", len(system), system)
		GlobalTraceLogger("User Prompt (len=%d):\n%s", len(user), user)
	}
	start := time.Now()
	resp, err := t.inner.GenerateText(ctx, system, user)
	latency := time.Since(start)
	if GlobalTraceLogger != nil {
		GlobalTraceLogger("LLM Latency: %v", latency)
		if err != nil {
			GlobalTraceLogger("LLM Error: %v", err)
		} else {
			GlobalTraceLogger("LLM Response (len=%d):\n%s", len(resp), resp)
		}
		GlobalTraceLogger("-------------------")
	}
	return resp, err
}

func (t *TraceProvider) Name() string {
	return t.inner.Name()
}

// Engine identifies which LLM backend to use.
type Engine string

const (
	EngineGemini Engine = "gemini"
	EngineOllama Engine = "ollama"
	// EngineFreeLLMAPI routes requests through the local freellmapi aggregator
	// (localhost:3001), which automatically falls back across 13 free LLM providers
	// (Groq, Gemini, Cerebras, Ollama Cloud, OpenRouter, etc.) with built-in
	// rate-limit tracking, sticky sessions, and penalty-based routing.
	EngineFreeLLMAPI Engine = "freellmapi"
	EngineOpenAI     Engine = "openai"
)

// NewProvider constructs the appropriate Provider from the application Config
// and the selected engine name.
func NewProvider(cfg *config.Config, engine Engine) (Provider, error) {
	var p Provider
	var err error

	switch engine {
	case EngineGemini:
		p, err = NewGeminiClient(GeminiConfig{
			APIKey: cfg.GeminiAPIKey,
			Model:  cfg.GeminiModel,
		})
	case EngineOllama:
		p, err = NewOllamaClient(OllamaConfig{
			BaseURL:   cfg.OllamaBaseURL,
			Model:     cfg.OllamaModel,
			TimeoutMS: cfg.OllamaTimeoutMS,
			APIKey:    cfg.LLMAPIKey,
		})
	case EngineFreeLLMAPI:
		model := cfg.FreeLLMAPIModel
		if model == "" {
			model = "auto"
		}

		p, err = NewOllamaClient(OllamaConfig{
			BaseURL:     cfg.FreeLLMAPIBaseURL + "/v1",
			Model:       model,
			TimeoutMS:   cfg.FreeLLMAPITimeoutMS,
			APIKey:      cfg.LLMAPIKey,
			AllowRemote: true,
		})

	case EngineOpenAI:
		// OpenAI requires the API key and model name to be set
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			apiKey = cfg.LLMAPIKey
		}
		if apiKey == "" {
			return nil, fmt.Errorf("LLM_API_KEY or OPENAI_API_KEY is required for OpenAI engine")
		}

		model := cfg.OpenAIModel
		if model == "" || model == "auto" {
			model = "gpt-4o-mini"
		}

		p, err = NewOllamaClient(OllamaConfig{
			BaseURL:     "https://api.openai.com/v1",
			Model:       model,
			TimeoutMS:   cfg.OpenAITimeoutMS,
			APIKey:      apiKey,
			AllowRemote: true,
		})

	default:
		return nil, fmt.Errorf("unknown LLM engine %q: choose 'gemini', 'ollama', 'freellmapi', or 'openai'", engine)
	}

	if err != nil {
		return nil, err
	}

	if GlobalTraceLogger != nil {
		return &TraceProvider{inner: p}, nil
	}

	return p, nil
}
