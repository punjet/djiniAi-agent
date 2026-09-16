package client

import (
	"djinni-bot-go/internal/config"
	"github.com/imroc/req/v3"
)

// ExaClient wraps req.Client for Exa.ai API interactions
type ExaClient struct {
	Client *req.Client
	Config *config.Config
}

// NewExaClient initializes the Exa client with API key and base URL
func NewExaClient(cfg *config.Config) *ExaClient {
	c := req.NewClient()
	c.SetCommonHeader("Authorization", "Bearer "+cfg.ExaAPIKey)
	c.SetBaseURL("https://api.exa.ai/v1")

	return &ExaClient{
		Client: c,
		Config: cfg,
	}
}

// Search performs a web search and returns Markdown results
func (e *ExaClient) Search(query string) (string, error) {
	var resp struct {
		Markdown string `json:"markdown"`
	}

	var payload = map[string]interface{}{
		"query": query,
	}

	req := e.Client.R().
		SetBody(payload).
		SetSuccessResult(&resp)
	_, err := req.Post("/search")
	if err != nil {
		return "", err
	}

	return resp.Markdown, nil
}

