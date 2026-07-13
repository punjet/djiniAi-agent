package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	SessionID           string
	CSRFToken           string
	GeminiAPIKey        string
	GeminiModel         string
	OllamaModel         string
	OllamaBaseURL       string
	OllamaTimeoutMS     int
	LLMAPIKey           string
	// FreeLLMAPI — local OpenAI-compatible aggregator (freellmapi project).
	// Runs on localhost:3001 and provides automatic fallback across 13 free LLM providers.
	FreeLLMAPIBaseURL   string // default: http://localhost:3001
	FreeLLMAPIModel     string // default: "" (freellmapi auto-routes to the best available model)
	FreeLLMAPITimeoutMS int    // default: 300000 (reasoning models can take 30-90s)

	OpenAIModel     string // default: "gpt-4o-mini" (falls back to FREELLMAPI_MODEL → LLM_MODEL)
	OpenAITimeoutMS int    // default: 300000
}

// loadEnvDefaults reads all env vars, applies defaults, and returns a populated Config.
// Shared by LoadConfig and MustLoadPartial to keep both in sync without duplication.
func loadEnvDefaults() *Config {
	geminiModel := os.Getenv("GEMINI_MODEL")
	if geminiModel == "" {
		geminiModel = "gemini-2.5-flash"
	}

	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaModel == "" {
		ollamaModel = os.Getenv("LLM_MODEL")
	}
	if ollamaModel == "" {
		ollamaModel = "llama3.3"
	}

	ollamaBaseURL := os.Getenv("OLLAMA_BASE_URL")
	if ollamaBaseURL == "" {
		ollamaBaseURL = "http://localhost:11434"
	}

	ollamaTimeoutMSStr := os.Getenv("OLLAMA_TIMEOUT_MS")
	ollamaTimeoutMS := 300_000
	if ollamaTimeoutMSStr != "" {
		if val, err := strconv.Atoi(ollamaTimeoutMSStr); err == nil {
			ollamaTimeoutMS = val
		}
	}

	freellmAPIBaseURL := os.Getenv("FREELLMAPI_BASE_URL")
	if freellmAPIBaseURL == "" {
		freellmAPIBaseURL = os.Getenv("LLM_BASE_URL")
	}
	if freellmAPIBaseURL == "" {
		freellmAPIBaseURL = "http://localhost:3001"
	}
	freellmAPIBaseURL = strings.TrimSuffix(freellmAPIBaseURL, "/v1")
	freellmAPIBaseURL = strings.TrimSuffix(freellmAPIBaseURL, "/v1/")
	freellmAPIBaseURL = strings.TrimRight(freellmAPIBaseURL, "/")

	freellmAPIModel := os.Getenv("FREELLMAPI_MODEL")
	if freellmAPIModel == "" {
		freellmAPIModel = os.Getenv("LLM_MODEL")
	}

	freellmAPITimeoutMSStr := os.Getenv("FREELLMAPI_TIMEOUT_MS")
	freellmAPITimeoutMS := 300_000
	if freellmAPITimeoutMSStr != "" {
		if val, err := strconv.Atoi(freellmAPITimeoutMSStr); err == nil {
			freellmAPITimeoutMS = val
		}
	}

	openAIModel := os.Getenv("OPENAI_MODEL")
	if openAIModel == "" {
		openAIModel = os.Getenv("FREELLMAPI_MODEL")
	}
	if openAIModel == "" {
		openAIModel = os.Getenv("LLM_MODEL")
	}
	if openAIModel == "" {
		openAIModel = "gpt-4o-mini"
	}

	openAITimeoutMSStr := os.Getenv("OPENAI_TIMEOUT_MS")
	openAITimeoutMS := 300_000
	if openAITimeoutMSStr != "" {
		if val, err := strconv.Atoi(openAITimeoutMSStr); err == nil {
			openAITimeoutMS = val
		}
	}

	return &Config{
		SessionID:           os.Getenv("DJINNI_SESSIONID"),
		CSRFToken:           os.Getenv("DJINNI_CSRFTOKEN"),
		GeminiAPIKey:        os.Getenv("GEMINI_API_KEY"),
		GeminiModel:         geminiModel,
		OllamaModel:         ollamaModel,
		OllamaBaseURL:       ollamaBaseURL,
		OllamaTimeoutMS:     ollamaTimeoutMS,
		LLMAPIKey:           os.Getenv("LLM_API_KEY"),
		FreeLLMAPIBaseURL:   freellmAPIBaseURL,
		FreeLLMAPIModel:     freellmAPIModel,
		FreeLLMAPITimeoutMS: freellmAPITimeoutMS,
		OpenAIModel:         openAIModel,
		OpenAITimeoutMS:     openAITimeoutMS,
	}
}

// LoadConfig loads the configuration from the environment and validates it.
// Requires Djinni session credentials — use MustLoadPartial for career-ops commands.
func LoadConfig() (*Config, error) {
	// Attempt to load .env file if it exists.
	// We ignore the error since variables can be set directly in the environment.
	_ = godotenv.Load()

	cfg := loadEnvDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// djinniCookieHelp guides users through extracting session cookies from the
// Djinni website when required env vars are missing.
const djinniCookieHelp = `
To get these values:
  1. Log in to https://djinni.co in your browser
  2. Open DevTools (F12) → Application (Chrome) or Storage (Firefox) → Cookies
  3. Copy the "sessionid" and "csrftoken" cookie values
  4. Set them as environment variables:
     export DJINNI_SESSIONID="<value>"
     export DJINNI_CSRFTOKEN="<value>"
  Or add them to a .env file in the project root:
     DJINNI_SESSIONID="<value>"
     DJINNI_CSRFTOKEN="<value>"
`

// Validate checks if all required configuration parameters are present.
// This is the strict validation used by the Djinni bot — requires session cookies.
func (c *Config) Validate() error {
	if c.SessionID == "" {
		return errors.New("DJINNI_SESSIONID is missing" + djinniCookieHelp)
	}
	if c.CSRFToken == "" {
		return errors.New("DJINNI_CSRFTOKEN is missing" + djinniCookieHelp)
	}
	return nil
}

// MustLoadPartial loads configuration from the environment without requiring
// Djinni session credentials. Use this for career-ops subcommands that only
// need LLM keys (GEMINI_API_KEY, OLLAMA_*, FREELLMAPI_*) and do not call the Djinni API.
func MustLoadPartial() *Config {
	_ = godotenv.Load()
	return loadEnvDefaults()
}
