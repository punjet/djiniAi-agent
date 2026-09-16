package agent

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"djinni-bot-go/internal/config"
	mcp_server "djinni-bot-go/internal/mcp"

	metorostdio "github.com/metoro-io/mcp-golang/transport/stdio"
	modelmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

// MCPIntegrationMode defines how to connect to the MCP server.
type MCPIntegrationMode string

const (
	// ModeStdio connects to an external MCP server process via stdio.
	ModeStdio MCPIntegrationMode = "stdio"
	// ModeInProcess bridges the local MCP server logic (or Go functions) directly to ADK tools.
	ModeInProcess MCPIntegrationMode = "in_process"
)

// LoadMCPTools creates an ADK Toolset from the local MCP server tools.
func LoadMCPTools(ctx context.Context, mode MCPIntegrationMode, cfg *config.Config, serverCommand string) (tool.Toolset, error) {
	switch mode {
	case ModeStdio:
		return loadStdioMCPTools(ctx, serverCommand)
	case ModeInProcess:
		return loadInProcessMCPTools(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown integration mode: %s", mode)
	}
}

// loadStdioMCPTools connects to an external MCP server binary using a CommandTransport.
func loadStdioMCPTools(ctx context.Context, command string) (tool.Toolset, error) {
	if command == "" {
		command = "mcp-server" // default binary name
	}
	transport := &modelmcp.CommandTransport{Command: exec.Command(command)}
	
	ts, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP toolset via stdio: %w", err)
	}
	
	return ts, nil
}

// loadInProcessMCPTools runs the MCP server in a goroutine and connects to it via io.Pipe.
func loadInProcessMCPTools(ctx context.Context, cfg *config.Config) (tool.Toolset, error) {
	// Create pipes for bidirectional communication
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	// 1. Setup metoro server using the custom NewServerWithTransport function
	metoroTransport := metorostdio.NewStdioServerTransportWithIO(serverReader, serverWriter)
	server, err := mcp_server.NewServerWithTransport(cfg, metoroTransport)
	if err != nil {
		return nil, fmt.Errorf("failed to create in-process MCP server: %w", err)
	}

	go func() {
		if err := server.Serve(); err != nil {
			fmt.Printf("In-process MCP server stopped: %v\n", err)
		}
	}()

	// 2. Setup modelcontextprotocol client
	transport := &modelmcp.IOTransport{
		Reader: clientReader,
		Writer: clientWriter,
	}
	
	ts, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP toolset via in-process pipe: %w", err)
	}

	return ts, nil
}
