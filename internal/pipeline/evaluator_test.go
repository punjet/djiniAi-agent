package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	
	"djinni-bot-go/internal/llm"
)

func TestEvaluateJob_Placeholder(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "cv.md"), []byte("CV Content"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := &llm.MockProvider{
		GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
			return "MOCK", nil
		},
		ProviderName: "MockLLM",
	}
	
	if mock.Name() != "MockLLM" {
		t.Errorf("expected MockLLM, got %s", mock.Name())
	}
}
