package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/model/openaimodel"
)

func TestNewEvaluatorAgent(t *testing.T) {
	ctx := context.Background()
	cfg := &openaimodel.ClientConfig{APIKey: "test-key"}
	llm, err := openaimodel.NewModel(ctx, "gpt-4o-mini", cfg)
	require.NoError(t, err)

	ag, err := NewEvaluatorAgent(llm, nil)
	require.NoError(t, err)
	require.NotNil(t, ag)
	require.Equal(t, "EvaluatorAgent", ag.Name())
}
