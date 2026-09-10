package llm

import "context"

// MockProvider is a mock implementation of Provider for testing purposes.
type MockProvider struct {
	GenerateTextFunc   func(ctx context.Context, system, user string) (string, error)
	GenerateEmbeddingFunc func(ctx context.Context, text string) ([]float32, error)
	ProviderName       string
}

// GenerateText calls GenerateTextFunc if defined, otherwise returns empty string.
func (m *MockProvider) GenerateText(ctx context.Context, system, user string) (string, error) {
	if m.GenerateTextFunc != nil {
		return m.GenerateTextFunc(ctx, system, user)
	}
	return "", nil
}

// GenerateEmbedding calls GenerateEmbeddingFunc if defined, otherwise returns nil.
func (m *MockProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if m.GenerateEmbeddingFunc != nil {
		return m.GenerateEmbeddingFunc(ctx, text)
	}
	return nil, nil
}

// Name returns ProviderName if defined, otherwise "MockProvider".
func (m *MockProvider) Name() string {
	if m.ProviderName != "" {
		return m.ProviderName
	}
	return "MockProvider"
}
