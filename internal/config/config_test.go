package config

import (
	"os"
	"strings"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid config",
			config: Config{
				SessionID: "session123",
				CSRFToken: "token123",
			},
			expectErr: false,
		},
		{
			name: "missing session id",
			config: Config{
				CSRFToken: "token123",
			},
			expectErr: true,
			errMsg:    "DJINNI_SESSIONID is missing",
		},
		{
			name: "missing csrf token",
			config: Config{
				SessionID: "session123",
			},
			expectErr: true,
			errMsg:    "DJINNI_CSRFTOKEN is missing",
		},
		{
			name:      "missing both",
			config:    Config{},
			expectErr: true,
			errMsg:    "DJINNI_SESSIONID is missing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.expectErr {
				t.Fatalf("expected error: %v, got: %v", tt.expectErr, err)
			}
			if err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error message containing %q, got: %s", tt.errMsg, err.Error())
			}
			// Verify help text is included in error message
			if err != nil && tt.expectErr {
				if !strings.Contains(err.Error(), "djinni.co") {
					t.Errorf("error message should contain setup instructions")
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Backup original env vars
	origSession := os.Getenv("DJINNI_SESSIONID")
	origCSRF := os.Getenv("DJINNI_CSRFTOKEN")
	defer func() {
		os.Setenv("DJINNI_SESSIONID", origSession)
		os.Setenv("DJINNI_CSRFTOKEN", origCSRF)
	}()

	t.Run("successful load", func(t *testing.T) {
		t.Setenv("DJINNI_SESSIONID", "sess-test")
		t.Setenv("DJINNI_CSRFTOKEN", "csrf-test")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("unexpected error loading config: %v", err)
		}

		if cfg.SessionID != "sess-test" {
			t.Errorf("expected SessionID sess-test, got %s", cfg.SessionID)
		}
		if cfg.CSRFToken != "csrf-test" {
			t.Errorf("expected CSRFToken csrf-test, got %s", cfg.CSRFToken)
		}
	})

	t.Run("missing only session id", func(t *testing.T) {
		os.Unsetenv("DJINNI_SESSIONID")
		t.Setenv("DJINNI_CSRFTOKEN", "csrf-test")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "DJINNI_SESSIONID is missing") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("missing only csrf token", func(t *testing.T) {
		t.Setenv("DJINNI_SESSIONID", "sess-test")
		os.Unsetenv("DJINNI_CSRFTOKEN")

		_, err := LoadConfig()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "DJINNI_CSRFTOKEN is missing") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}

func TestMustLoadPartial(t *testing.T) {
	// Ensure we test the default fallback behavior
	os.Clearenv()
	t.Setenv("GEMINI_API_KEY", "test-gemini-key")
	t.Setenv("OLLAMA_TIMEOUT_MS", "invalid-number") // Should fallback to default
	t.Setenv("FREELLMAPI_BASE_URL", "http://custom:3001/v1/") // Should strip suffix

	cfg := MustLoadPartial()

	if cfg.GeminiModel != "gemini-2.5-flash" {
		t.Errorf("expected GeminiModel gemini-2.5-flash, got %s", cfg.GeminiModel)
	}
	if cfg.OllamaModel != "llama3.3" {
		t.Errorf("expected OllamaModel llama3.3, got %s", cfg.OllamaModel)
	}
	if cfg.OllamaBaseURL != "http://localhost:11434" {
		t.Errorf("expected OllamaBaseURL http://localhost:11434, got %s", cfg.OllamaBaseURL)
	}
	if cfg.OllamaTimeoutMS != 300000 {
		t.Errorf("expected OllamaTimeoutMS 300000, got %d", cfg.OllamaTimeoutMS)
	}
	if cfg.GeminiAPIKey != "test-gemini-key" {
		t.Errorf("expected GeminiAPIKey test-gemini-key, got %s", cfg.GeminiAPIKey)
	}
	if cfg.FreeLLMAPIBaseURL != "http://custom:3001" {
		t.Errorf("expected FreeLLMAPIBaseURL http://custom:3001, got %s", cfg.FreeLLMAPIBaseURL)
	}

	// Test overrides
	t.Setenv("GEMINI_MODEL", "gemini-pro")
	t.Setenv("LLM_MODEL", "custom-model")
	t.Setenv("OLLAMA_TIMEOUT_MS", "150000")

	cfg2 := MustLoadPartial()
	if cfg2.GeminiModel != "gemini-pro" {
		t.Errorf("expected GeminiModel gemini-pro, got %s", cfg2.GeminiModel)
	}
	// OLLAMA_MODEL falls back to LLM_MODEL
	if cfg2.OllamaModel != "custom-model" {
		t.Errorf("expected OllamaModel custom-model, got %s", cfg2.OllamaModel)
	}
	if cfg2.OllamaTimeoutMS != 150000 {
		t.Errorf("expected OllamaTimeoutMS 150000, got %d", cfg2.OllamaTimeoutMS)
	}
	if cfg2.FreeLLMAPIModel != "custom-model" {
		t.Errorf("expected FreeLLMAPIModel custom-model, got %s", cfg2.FreeLLMAPIModel)
	}
}

func TestOpenAIVars(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		os.Clearenv()
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("DJINNI_SESSIONID", "test-session")
		t.Setenv("DJINNI_CSRFTOKEN", "test-csrf")

		cfg := MustLoadPartial()

		if cfg.OpenAIModel != "gpt-5-mini" {
			t.Errorf("expected OpenAIModel gpt-5-mini, got %s", cfg.OpenAIModel)
		}
		if cfg.OpenAITimeoutMS != 300000 {
			t.Errorf("expected OpenAITimeoutMS 300000, got %d", cfg.OpenAITimeoutMS)
		}
	})

	t.Run("OPENAI_MODEL env override", func(t *testing.T) {
		os.Clearenv()
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("OPENAI_MODEL", "gpt-4o")

		cfg := MustLoadPartial()

		if cfg.OpenAIModel != "gpt-4o" {
			t.Errorf("expected OpenAIModel gpt-4o, got %s", cfg.OpenAIModel)
		}
	})

	t.Run("OPENAI_MODEL falls back to FREELLMAPI_MODEL", func(t *testing.T) {
		os.Clearenv()
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("FREELLMAPI_MODEL", "free-model")

		cfg := MustLoadPartial()

		if cfg.OpenAIModel != "free-model" {
			t.Errorf("expected OpenAIModel free-model, got %s", cfg.OpenAIModel)
		}
	})

	t.Run("OPENAI_MODEL falls back to LLM_MODEL", func(t *testing.T) {
		os.Clearenv()
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("LLM_MODEL", "llm-model")

		cfg := MustLoadPartial()

		if cfg.OpenAIModel != "llm-model" {
			t.Errorf("expected OpenAIModel llm-model, got %s", cfg.OpenAIModel)
		}
	})

	t.Run("OPENAI_TIMEOUT_MS env override", func(t *testing.T) {
		os.Clearenv()
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("OPENAI_TIMEOUT_MS", "60000")

		cfg := MustLoadPartial()

		if cfg.OpenAITimeoutMS != 60000 {
			t.Errorf("expected OpenAITimeoutMS 60000, got %d", cfg.OpenAITimeoutMS)
		}
	})

	t.Run("OPENAI_TIMEOUT_MS invalid falls back to default", func(t *testing.T) {
		os.Clearenv()
		t.Setenv("GEMINI_API_KEY", "test-gemini-key")
		t.Setenv("OPENAI_TIMEOUT_MS", "invalid")

		cfg := MustLoadPartial()

		if cfg.OpenAITimeoutMS != 300000 {
			t.Errorf("expected OpenAITimeoutMS 300000, got %d", cfg.OpenAITimeoutMS)
		}
	})
}
