package api

import (
	"strings"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/logger"
)

// CheckToken verifies if the current session token is valid by hitting the dashboard.
func CheckToken(dc *client.DjinniClient) bool {
	logger.Log.Info("Starting token validation", "operation", "CheckToken")


	targetURL := "https://djinni.co/my/dashboard/"
	if dc.Client.BaseURL != "" {
		targetURL = dc.Client.BaseURL + "/my/dashboard/"
	}

	resp, err := dc.Client.R().Get(targetURL)
	if err != nil {
		return false
	}
	
	finalURL := ""
	if resp.Response != nil && resp.Response.Request != nil && resp.Response.Request.URL != nil {
		finalURL = resp.Response.Request.URL.String()
	}

	if strings.Contains(finalURL, "/login") {
		return false
	}

	// Also check body for login forms just in case
	if strings.Contains(resp.String(), `action="/login"`) || strings.Contains(resp.String(), "Sign in to Djinni") {
		return false
	}

	return true
}
