package mcp_server

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_Registration(t *testing.T) {
	server, err := NewServer(nil)
	require.NoError(t, err)
	require.NotNil(t, server)

	// Since we mock the execution, let's just invoke the extraction tool directly to verify it runs
	htmlContent := `
	<html>
		<body>
			<h1 class="job-details--title">Senior Go Developer</h1>
			<div class="job-details--company_name">Tech Corp</div>
			<div class="job-post__description">We are looking for a Go developer.</div>
		</body>
	</html>
	`

	args := ExtractJobDetailsArgs{
		HtmlContent: htmlContent,
	}

	// Call the tool function directly
	resp, err := server.extractJobDetailsTool(context.Background(), args)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Ensure the response has text content containing the title and company
	require.Len(t, resp.Content, 1)
	textContent := resp.Content[0].TextContent
	require.NotNil(t, textContent)

	assert.True(t, strings.Contains(textContent.Text, "Senior Go Developer"), "Expected 'Senior Go Developer' in response")
	assert.True(t, strings.Contains(textContent.Text, "Tech Corp"), "Expected 'Tech Corp' in response")
}
