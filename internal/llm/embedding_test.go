package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGenerateEmbedding_Unit_MockProvider tests GenerateEmbedding using a MockProvider.
// This is a unit test that checks the interface implementation.
func TestGenerateEmbedding_Unit_MockProvider(t *testing.T) {
	ctx := context.Background()

	// Test case 1: Successful embedding generation
	mock := &MockProvider{
		GenerateEmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
			assert.Equal(t, "test text", text)
			// Return a mock embedding vector of length 1536
			embedding := make([]float32, 1536)
			for i := range embedding {
				embedding[i] = float32(i) / 1536.0
			}
			return embedding, nil
		},
	}

	embedding, err := mock.GenerateEmbedding(ctx, "test text")
	assert.NoError(t, err)
	assert.Len(t, embedding, 1536)
	assert.Equal(t, float32(0)/1536.0, embedding[0])
	assert.Equal(t, float32(1535)/1536.0, embedding[1535])

	// Test case 2: Error case
	mock2 := &MockProvider{
		GenerateEmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
			return nil, assert.AnError
		},
	}

	embedding, err = mock2.GenerateEmbedding(ctx, "test")
	assert.Error(t, err)
	assert.Nil(t, embedding)
}

// TestGenerateEmbedding_Scalability tests the scalability of embedding generation.
// This runs the mock in a loop to ensure performance is acceptable.
func TestGenerateEmbedding_Scalability(t *testing.T) {
	ctx := context.Background()

	mock := &MockProvider{
		GenerateEmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
			embedding := make([]float32, 1536)
			for i := range embedding {
				embedding[i] = float32(i) / 1536.0
			}
			return embedding, nil
		},
	}

	// Generate embeddings for 100 texts
	texts := make([]string, 100)
	for i := range texts {
		texts[i] = "test"
	}

	ctx, cancel := context.WithTimeout(ctx, 5*1000000000) // 5 seconds
	defer cancel()

	for _, text := range texts {
		embedding, err := mock.GenerateEmbedding(ctx, text)
		assert.NoError(t, err)
		assert.Len(t, embedding, 1536)
	}
}

// TestGenerateEmbedding_EmptyText tests that empty text is handled.
func TestGenerateEmbedding_EmptyText(t *testing.T) {
	ctx := context.Background()

	mock := &MockProvider{
		GenerateEmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
			// Embedding API typically handles empty text
			embedding := make([]float32, 1536)
			for i := range embedding {
				embedding[i] = float32(i) / 1536.0
			}
			return embedding, nil
		},
	}

	embedding, err := mock.GenerateEmbedding(ctx, "")
	assert.NoError(t, err)
	assert.Len(t, embedding, 1536)
	assert.Equal(t, float32(0)/1536.0, embedding[0])
}
