package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/logger"
)

// CheckToken verifies if the current session token is valid by hitting the dashboard.
// It retries up to 3 times to handle transient network errors, and distinguishes
// between "network failure" (returns true to avoid false-positive alerts) and
// "definitely logged out" (returns false).
func CheckToken(dc *client.DjinniClient) bool {
	logger.Log.Info("Starting token validation", "operation", "CheckToken")

	targetURL := "https://djinni.co/my/dashboard/"
	if dc.Client.BaseURL != "" {
		targetURL = dc.Client.BaseURL + "/my/dashboard/"
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			logger.Log.Info("Retrying token check", "attempt", attempt)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		resp, err := dc.Client.R().Get(targetURL)
		if err != nil {
			lastErr = err
			logger.Log.Warn("Token check request failed", "attempt", attempt, "error", err)
			continue
		}

		// Check HTTP status — anything >= 400 that isn't a redirect is suspicious
		statusCode := 0
		if resp.Response != nil {
			statusCode = resp.Response.StatusCode
		}

		// Djinni redirects to /login/ with 302; final URL after redirect is what matters
		finalURL := ""
		if resp.Response != nil && resp.Response.Request != nil && resp.Response.Request.URL != nil {
			finalURL = resp.Response.Request.URL.String()
		}

		body := resp.String()

		// Definitive signs of being logged out
		isLoggedOut := strings.Contains(finalURL, "/login") ||
			strings.Contains(body, `action="/login"`) ||
			strings.Contains(body, "Sign in to Djinni") ||
			strings.Contains(body, "Увійти до Djinni") ||
			statusCode == http.StatusForbidden

		if isLoggedOut {
			logger.Log.Warn("Token validation failed: session expired or invalid",
				"finalURL", finalURL, "statusCode", statusCode)
			return false
		}

		// Definitive sign of being logged in: dashboard page loaded
		isLoggedIn := strings.Contains(finalURL, "/my/dashboard") ||
			strings.Contains(body, "my-profile") ||
			strings.Contains(body, "logout") ||
			strings.Contains(body, "Вийти")

		if isLoggedIn {
			logger.Log.Info("Token validation succeeded", "finalURL", finalURL)
			return true
		}

		// Ambiguous response — retry
		lastErr = fmt.Errorf("ambiguous response: status=%d url=%s", statusCode, finalURL)
		logger.Log.Warn("Ambiguous token check response, retrying", "attempt", attempt, "finalURL", finalURL)
	}

	// After all retries — if it's a network error, don't false-alarm
	if lastErr != nil {
		logger.Log.Error("Token check failed after all retries — treating as network error, not invalidating",
			"error", lastErr)
		// Return true to avoid spamming "session expired" on temporary network issues
		return true
	}

	return false
}
