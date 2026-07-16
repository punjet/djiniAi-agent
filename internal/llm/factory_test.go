package llm

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"djinni-bot-go/internal/config"
)

func TestNewProvider_ModelSelection(t *testing.T) {
	cfg := &config.Config{
		ResumeModel: "gpt-5-mini",
		EvalModel:   "gpt-5-nano",
		OpenAIModel: "gpt-4o-mini", // Default model for OpenAI engine
		LLMAPIKey:   "test-api-key",
	}

	// Test resume task
	provider, err := NewProvider(cfg, EngineOpenAI, "resume")
	assert.NoError(t, err)
	assert.Contains(t, provider.Name(), "Ollama (gpt-5-mini)", "Resume task should use gpt-5-mini")

	// Test evaluation task
	provider, err = NewProvider(cfg, EngineOpenAI, "evaluation")
	assert.NoError(t, err)
	assert.Contains(t, provider.Name(), "Ollama (gpt-5-nano)", "Evaluation task should use gpt-5-nano")

	// Test default case for unknown task
	provider, err = NewProvider(cfg, EngineOpenAI, "unknown_task")
	assert.NoError(t, err)
	assert.Contains(t, provider.Name(), "Ollama (gpt-4o-mini)", "Unknown task should use default OpenAI model")
}
