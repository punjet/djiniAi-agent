package llm

import (
	"os"
	"testing"
)

func TestNewOllamaClient_LoopbackGuard_Blocks(t *testing.T) {
	// Ensure LLM_BASE_URL and OLLAMA_ALLOW_REMOTE are not set
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("OLLAMA_ALLOW_REMOTE")

	_, err := NewOllamaClient(OllamaConfig{
		BaseURL: "http://remote-server.example.com:11434",
		Model:   "llama3.3",
	})
	if err == nil {
		t.Fatal("expected loopback guard to block remote URL, got nil error")
	}
}

func TestNewOllamaClient_LoopbackGuard_AllowsLocalhost(t *testing.T) {
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("OLLAMA_ALLOW_REMOTE")

	_, err := NewOllamaClient(OllamaConfig{
		BaseURL: "http://localhost:11434",
		Model:   "llama3.3",
	})
	if err != nil {
		t.Fatalf("expected localhost to pass loopback guard, got: %v", err)
	}
}

func TestNewOllamaClient_LoopbackGuard_AllowsRemoteWithEnvVar(t *testing.T) {
	os.Unsetenv("LLM_BASE_URL")
	os.Setenv("OLLAMA_ALLOW_REMOTE", "1")
	defer os.Unsetenv("OLLAMA_ALLOW_REMOTE")

	_, err := NewOllamaClient(OllamaConfig{
		BaseURL: "http://remote-server.example.com:11434",
		Model:   "llama3.3",
	})
	if err != nil {
		t.Fatalf("expected OLLAMA_ALLOW_REMOTE=1 to bypass guard, got: %v", err)
	}
}

func TestNewOllamaClient_LoopbackGuard_AllowsLLMBaseURLOverride(t *testing.T) {
	// When LLM_BASE_URL is set the guard is skipped entirely
	os.Setenv("LLM_BASE_URL", "http://some-compatible-api.example.com")
	defer os.Unsetenv("LLM_BASE_URL")

	_, err := NewOllamaClient(OllamaConfig{
		BaseURL: "http://some-compatible-api.example.com",
		Model:   "gpt-4o",
	})
	if err != nil {
		t.Fatalf("expected LLM_BASE_URL override to skip guard, got: %v", err)
	}
}

func TestNewOllamaClient_Defaults(t *testing.T) {
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("OLLAMA_ALLOW_REMOTE")

	client, err := NewOllamaClient(OllamaConfig{})
	if err != nil {
		t.Fatalf("unexpected error with defaults: %v", err)
	}
	if client.model != "llama3.3" {
		t.Errorf("default model: got %q, want %q", client.model, "llama3.3")
	}
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("default baseURL: got %q, want %q", client.baseURL, "http://localhost:11434")
	}
	if client.timeoutMS != 300_000 {
		t.Errorf("default timeoutMS: got %d, want 300000", client.timeoutMS)
	}
}

func TestOllamaClient_Name(t *testing.T) {
	os.Unsetenv("LLM_BASE_URL")
	client, err := NewOllamaClient(OllamaConfig{Model: "qwen2.5:72b"})
	if err != nil {
		t.Fatal(err)
	}
	got := client.Name()
	want := "Ollama (qwen2.5:72b)"
	if got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}
