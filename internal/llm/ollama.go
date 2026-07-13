package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// OllamaClient implements Provider by calling a local (or tunnelled) Ollama server
// via the OpenAI-compatible /v1/chat/completions endpoint.
type OllamaClient struct {
	baseURL    string
	model      string
	timeoutMS  int
	apiKey     string // optional bearer token (for remote/tunnelled endpoints)
	httpClient *http.Client
}

// OllamaConfig holds configuration for the Ollama client.
type OllamaConfig struct {
	BaseURL     string // default: "http://localhost:11434"
	Model       string // default: "llama3.3"
	TimeoutMS   int    // default: 300000 (5 min)
	APIKey      string // optional
	AllowRemote bool   // skip the loopback guard (for compatible remote APIs)
}

// NewOllamaClient creates a new OllamaClient and runs a loopback guard
// (mirrors the security check in ollama-eval.mjs).
func NewOllamaClient(cfg OllamaConfig) (*OllamaClient, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	// Trim trailing slash for consistent URL construction
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if cfg.Model == "auto" {
		cfg.Model = ""
	} else if cfg.Model == "" {
		cfg.Model = "llama3.3"
	}
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = 300_000
	}

	if !cfg.AllowRemote {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Ollama base URL %q: %w", cfg.BaseURL, err)
		}
		host := u.Hostname()
		isLoopback := host == "localhost" || host == "127.0.0.1" || host == "::1"
		if !isLoopback && os.Getenv("OLLAMA_ALLOW_REMOTE") != "1" {
			return nil, fmt.Errorf(
				"remote Ollama endpoint detected: %s\n"+
					"Your CV and job description would be sent to a remote server.\n"+
					"Set OLLAMA_ALLOW_REMOTE=1 to allow this intentionally.",
				cfg.BaseURL,
			)
		}
	}

	return &OllamaClient{
		baseURL:   cfg.BaseURL,
		model:     cfg.Model,
		timeoutMS: cfg.TimeoutMS,
		apiKey:    cfg.APIKey,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond,
		},
	}, nil
}

// Name implements Provider.
func (o *OllamaClient) Name() string {
	return fmt.Sprintf("Ollama (%s)", o.model)
}

// Probe checks whether the server is reachable.
// It tries /api/ping first (freellmapi endpoint), then falls back to /api/tags (native Ollama).
// Call this before GenerateText to give the user a clear error message early.
func (o *OllamaClient) Probe(ctx context.Context) error {
	// Try freellmapi's /api/ping first (returns {"status":"ok"})
	for _, suffix := range []string{"/api/ping", "/api/tags"} {
		probeURL := o.baseURL + suffix
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			continue
		}
		resp, err := o.httpClient.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}
	return fmt.Errorf("server not reachable at %s: %w\n"+
		"  1. Install Ollama: https://ollama.com  OR  start freellmapi: npm run dev\n"+
		"  2. Start server:   ollama serve\n"+
		"  3. Pull a model:   ollama pull %s",
		o.baseURL, fmt.Errorf("no healthy endpoint found"), o.model)
}

// chatRequest mirrors the OpenAI-compatible chat request body.
type chatRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []chatMessage `json:"messages"`
	Stream      bool          `json:"stream"`
	Options     *chatOptions  `json:"options,omitempty"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatOptions struct {
	NumCtx int `json:"num_ctx"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// GenerateText implements Provider.
func (o *OllamaClient) GenerateText(ctx context.Context, system, user string) (string, error) {
	// Build the endpoint (works for both /v1 and plain base URL)
	endpoint := o.baseURL + "/v1/chat/completions"
	if strings.HasSuffix(o.baseURL, "/v1") {
		endpoint = o.baseURL + "/chat/completions"
	}

	payload := chatRequest{
		Model: o.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Stream:      false,
		Temperature: 0.4,
	}
	if !strings.Contains(o.baseURL, "api.openai.com") {
		payload.Options = &chatOptions{NumCtx: 32768}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to build Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return "", fmt.Errorf("Ollama API error: HTTP %d — %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("Ollama returned an empty response")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}
