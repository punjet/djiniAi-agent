package client

import (
	"net/http"

	"djinni-bot-go/internal/config"

	"github.com/imroc/req/v3"
)

// DjinniClient wraps req.Client to interact with Djinni.co.
type DjinniClient struct {
	Client *req.Client
	Config *config.Config
}

// NewDjinniClient initializes a req.Client with Chrome impersonation, default headers, and session cookies.
func NewDjinniClient(cfg *config.Config) *DjinniClient {
	c := req.NewClient()
	c.ImpersonateChrome()

	// Set common headers
	c.SetCommonHeaders(map[string]string{
		"Referer":     "https://djinni.co",
		"Origin":      "https://djinni.co",
		"X-Csrftoken": cfg.CSRFToken,
	})

	// Set session cookies
	sessionCookie := &http.Cookie{
		Name:   "sessionid",
		Value:  cfg.SessionID,
		Domain: "djinni.co",
		Path:   "/",
	}
	csrfCookie := &http.Cookie{
		Name:   "csrftoken",
		Value:  cfg.CSRFToken,
		Domain: "djinni.co",
		Path:   "/",
	}
	c.SetCommonCookies(sessionCookie, csrfCookie)

	return &DjinniClient{
		Client: c,
		Config: cfg,
	}
}
