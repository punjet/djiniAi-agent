package agent

import (
	"context"
	"testing"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockContext implements agent.ReadonlyContext
type mockContext struct {
	agent.StrictContextMock
}

func TestLoadMCPToolsInProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Load the toolset
	ts, err := LoadMCPTools(ctx, ModeInProcess, nil, "")
	require.NoError(t, err, "Should load in-process tools without error")
	require.NotNil(t, ts, "Toolset should not be nil")

	// Get tools from the toolset
	mCtx := &mockContext{agent.NewStrictContextMock(ctx)}
	tools, err := ts.Tools(mCtx)
	require.NoError(t, err)
	
	// We expect 2 tools: scrape_djinni and extract_djinni_job_details
	require.Len(t, tools, 2, "Should discover 2 tools")
	
	names := []string{tools[0].Name(), tools[1].Name()}
	assert.Contains(t, names, "scrape_djinni")
	assert.Contains(t, names, "extract_djinni_job_details")
}

func TestInvokeMCPTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := LoadMCPTools(ctx, ModeInProcess, nil, "")
	require.NoError(t, err)

	mCtx := &mockContext{agent.NewStrictContextMock(ctx)}
	tools, err := ts.Tools(mCtx)
	require.NoError(t, err)

	var scrapeTool tool.Tool
	for _, tl := range tools {
		if tl.Name() == "scrape_djinni" {
			scrapeTool = tl
		}
	}
	require.NotNil(t, scrapeTool, "Tool scrape_djinni not found")
}
