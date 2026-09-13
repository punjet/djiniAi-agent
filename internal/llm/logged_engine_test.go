package llm

import (
	"context"
	"errors"
	"testing"
)

func TestLoggedProvider_Name(t *testing.T) {
	mock := &MockProvider{
		ProviderName: "TestMockProvider",
	}

	logged := NewLoggedProvider(mock)
	if logged.Name() != "TestMockProvider" {
		t.Errorf("expected provider name 'TestMockProvider', got '%s'", logged.Name())
	}
}

func TestLoggedProvider_GenerateText(t *testing.T) {
	t.Run("success without LOG_DEEP_FULL", func(t *testing.T) {
		called := false
		mock := &MockProvider{
			ProviderName: "TestMockProvider",
			GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
				called = true
				if system != "sys_prompt" || user != "user_msg" {
					t.Errorf("unexpected arguments: system=%s, user=%s", system, user)
				}
				return "generated response", nil
			},
		}

		logged := NewLoggedProvider(mock)
		resp, err := logged.GenerateText(context.Background(), "sys_prompt", "user_msg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Errorf("expected GenerateTextFunc to be called")
		}
		if resp != "generated response" {
			t.Errorf("expected response 'generated response', got '%s'", resp)
		}
	})

	t.Run("success with LOG_DEEP_FULL", func(t *testing.T) {
		t.Setenv("LOG_DEEP_FULL", "true")
		called := false
		mock := &MockProvider{
			ProviderName: "TestMockProvider",
			GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
				called = true
				return "generated response", nil
			},
		}

		logged := NewLoggedProvider(mock)
		resp, err := logged.GenerateText(context.Background(), "sys_prompt", "user_msg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Errorf("expected GenerateTextFunc to be called")
		}
		if resp != "generated response" {
			t.Errorf("expected response 'generated response', got '%s'", resp)
		}
	})

	t.Run("error propagation", func(t *testing.T) {
		expectedErr := errors.New("llm error")
		mock := &MockProvider{
			ProviderName: "TestMockProvider",
			GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
				return "", expectedErr
			},
		}

		logged := NewLoggedProvider(mock)
		resp, err := logged.GenerateText(context.Background(), "sys", "usr")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
		if resp != "" {
			t.Errorf("expected empty response on error, got '%s'", resp)
		}
	})
}

func TestLoggedProvider_GenerateEmbedding(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		called := false
		expectedEmb := []float32{0.1, 0.2, 0.3}
		mock := &MockProvider{
			ProviderName: "TestMockProvider",
			GenerateEmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
				called = true
				if text != "embedding text" {
					t.Errorf("unexpected text argument: %s", text)
				}
				return expectedEmb, nil
			},
		}

		logged := NewLoggedProvider(mock)
		emb, err := logged.GenerateEmbedding(context.Background(), "embedding text")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Errorf("expected GenerateEmbeddingFunc to be called")
		}
		if len(emb) != len(expectedEmb) {
			t.Errorf("expected embedding length %d, got %d", len(expectedEmb), len(emb))
		}
	})

	t.Run("error propagation", func(t *testing.T) {
		expectedErr := errors.New("embedding error")
		mock := &MockProvider{
			ProviderName: "TestMockProvider",
			GenerateEmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
				return nil, expectedErr
			},
		}

		logged := NewLoggedProvider(mock)
		emb, err := logged.GenerateEmbedding(context.Background(), "text")
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
		if emb != nil {
			t.Errorf("expected nil embedding on error, got %v", emb)
		}
	})
}
