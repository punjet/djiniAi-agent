package mcp_server // renaming to avoid conflict with mcp import

import (
	"context"
	"encoding/json"
	"fmt"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/extractor"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport"
	"github.com/metoro-io/mcp-golang/transport/stdio"
)

// DjinniMCPServer wraps the Djinni API logic into an MCP Server.
type DjinniMCPServer struct {
	server       *mcp.Server
	djinniClient *client.DjinniClient
}

// ScrapeJobDetailsArgs defines the arguments for scrape_djinni tool.
type ScrapeJobDetailsArgs struct {
	JobSlug string `json:"jobSlug" jsonschema:"description=The Djinni job slug, e.g. 12345-senior-go-developer"`
}

// ExtractJobDetailsArgs defines the arguments for extract_djinni_job_details tool.
type ExtractJobDetailsArgs struct {
	HtmlContent string `json:"htmlContent" jsonschema:"description=The raw HTML content of the Djinni job page"`
}

// NewServer initializes the MCP server and registers tools using standard STDIO transport.
func NewServer(cfg *config.Config) (*DjinniMCPServer, error) {
	return NewServerWithTransport(cfg, stdio.NewStdioServerTransport())
}

// NewServerWithTransport initializes the MCP server and registers tools using a custom transport.
func NewServerWithTransport(cfg *config.Config, tr transport.Transport) (*DjinniMCPServer, error) {
	var djinniClient *client.DjinniClient
	if cfg != nil {
		djinniClient = client.NewDjinniClient(cfg)
	}

	server := mcp.NewServer(tr, mcp.WithName("DjinniExtractor"), mcp.WithVersion("1.0.0"))

	dms := &DjinniMCPServer{
		server:       server,
		djinniClient: djinniClient,
	}

	// Register tools
	err := server.RegisterTool("scrape_djinni", "Fetches and extracts job details from Djinni by job slug", dms.scrapeDjinniTool)
	if err != nil {
		return nil, fmt.Errorf("failed to register scrape_djinni: %w", err)
	}

	err = server.RegisterTool("extract_djinni_job_details", "Extracts job details from raw Djinni job HTML content", dms.extractJobDetailsTool)
	if err != nil {
		return nil, fmt.Errorf("failed to register extract_djinni_job_details: %w", err)
	}

	return dms, nil
}

// Serve runs the server using STDIO transport.
func (s *DjinniMCPServer) Serve() error {
	return s.server.Serve()
}

// scrapeDjinniTool handles the 'scrape_djinni' tool invocation.
func (s *DjinniMCPServer) scrapeDjinniTool(ctx context.Context, args ScrapeJobDetailsArgs) (*mcp.ToolResponse, error) {
	if args.JobSlug == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("Error: jobSlug is required")), nil
	}

	if s.djinniClient == nil {
		return mcp.NewToolResponse(mcp.NewTextContent("Error: DjinniClient is not configured (missing config/credentials)")), nil
	}

	jobDetails, err := api.GetJobDetails(s.djinniClient, args.JobSlug)
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Failed to scrape job details: %v", err))), nil
	}

	jsonBytes, err := json.MarshalIndent(jobDetails, "", "  ")
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Failed to encode job details: %v", err))), nil
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
}

// extractJobDetailsTool handles the 'extract_djinni_job_details' tool invocation.
func (s *DjinniMCPServer) extractJobDetailsTool(ctx context.Context, args ExtractJobDetailsArgs) (*mcp.ToolResponse, error) {
	if args.HtmlContent == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("Error: htmlContent is required")), nil
	}

	jobDetails, err := extractor.ExtractJobDetailsV2(args.HtmlContent)
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Failed to extract job details: %v", err))), nil
	}

	jsonBytes, err := json.MarshalIndent(jobDetails, "", "  ")
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("Failed to encode job details: %v", err))), nil
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
}
