package llm

import "context"

// MockProvider is a mock implementation of Provider for testing purposes.
type MockProvider struct {
	GenerateTextFunc func(ctx context.Context, system, user string) (string, error)
	ProviderName     string
}

// GenerateText calls GenerateTextFunc if defined, otherwise returns empty string.
func (m *MockProvider) GenerateText(ctx context.Context, system, user string) (string, error) {
	if m.GenerateTextFunc != nil {
		return m.GenerateTextFunc(ctx, system, user)
	}
	return "", nil
}

// Name returns ProviderName if defined, otherwise "MockProvider".
func (m *MockProvider) Name() string {
	if m.ProviderName != "" {
		return m.ProviderName
	}
	return "MockProvider"
}
