package main

import (
	"log"

	"djinni-bot-go/internal/config"
	mcp_server "djinni-bot-go/internal/mcp"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		// Log but don't fail immediately, some tools (like extract HTML) can work without credentials
		log.Printf("Warning: failed to load full config: %v", err)
	}

	server, err := mcp_server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize MCP server: %v", err)
	}

	if err := server.Serve(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
