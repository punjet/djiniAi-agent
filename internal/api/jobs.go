package api

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"os"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/extractor"
)

// GetDashboardJobs fetches /my/dashboard/ and parses it.
func GetDashboardJobs(dc *client.DjinniClient, page int) ([]extractor.JobSummary, error) {
	targetURL := "https://djinni.co/my/dashboard/"
	if dc.Client.BaseURL != "" {
		targetURL = dc.Client.BaseURL + "/my/dashboard/"
	}

	req := dc.Client.R().SetHeader("Referer", targetURL)
	if page > 1 {
		req.SetQueryParam("page", fmt.Sprintf("%d", page))
	}

	resp, err := req.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch dashboard jobs: %w", err)
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("unexpected status code fetching dashboard jobs: %d", resp.StatusCode)
	}

	return extractor.ExtractDashboardJobsV2(resp.String())
}

// SearchJobs fetches /jobs/ with queryParams and parses it.
func SearchJobs(dc *client.DjinniClient, queryParams map[string]string) ([]extractor.JobSummary, error) {
	targetURL := "https://djinni.co/jobs/"
	if dc.Client.BaseURL != "" {
		targetURL = dc.Client.BaseURL + "/jobs/"
	}

	resp, err := dc.Client.R().
		SetQueryParams(queryParams).
		SetHeader("Referer", targetURL).
		Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to search jobs: %w", err)
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("unexpected status code searching jobs: %d", resp.StatusCode)
	}

	return extractor.ExtractJobs(resp.String())
}

// GetJobDetails fetches the detailed job page /jobs/{jobSlug}/ and extracts job details.
func GetJobDetails(dc *client.DjinniClient, jobSlug string) (*JobFull, error) {
	if jobSlug == "" {
		return nil, errors.New("jobSlug cannot be empty")
	}

	targetURL := fmt.Sprintf("https://djinni.co/jobs/%s/", jobSlug)
	if dc.Client.BaseURL != "" {
		targetURL = fmt.Sprintf("%s/jobs/%s/", dc.Client.BaseURL, jobSlug)
	}

	resp, err := dc.Client.R().
		SetHeader("Referer", targetURL).
		Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job details: %w", err)
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("unexpected status code fetching job details: %d", resp.StatusCode)
	}

	details, err := extractor.ExtractJobDetailsV2(resp.String())
	if err != nil {
		return nil, fmt.Errorf("failed to extract job details: %w", err)
	}

	htmlStr := resp.String()
	alreadyApplied := extractor.IsAlreadyApplied(htmlStr)
	
	// FRAGILE: Relying on the presence of "js-inbox-toggle-reply-form" to determine if a job is applied or blocked.
	// TODO: Replace with a more robust structured check or API endpoint if available.
	if !alreadyApplied && !strings.Contains(htmlStr, "js-inbox-toggle-reply-form") && !strings.Contains(htmlStr, `<form action="?ref=for_me"`) {
		return nil, fmt.Errorf("job is strictly blocked by Djinni requirements or already applied (missing apply button, error=cant_apply) for URL: %s", targetURL)
	}

	// Extract ID from slug (first digits before first dash)
	parts := strings.Split(jobSlug, "-")
	jobID := ""
	if len(parts) > 0 {
		jobID = parts[0]
	}

	return &JobFull{
		ID:             jobID,
		Slug:           jobSlug,
		Title:          details.Title,
		Company:        details.Company,
		Description:    details.Description,
		Requirements:   details.Requirements,
		URL:            targetURL,
		QuizID:         details.QuizID,
		QuizQuestions:  details.QuizQuestions,
		AlreadyApplied: alreadyApplied,
	}, nil
}

// ApplyToJob submits a multipart POST request to /jobs/{jobSlug}/?ref=for_me.
// Returns success string if redirect to applied=ok is confirmed.
// extraFormData is merged into the POST form data (e.g. quiz answers).
func ApplyToJob(dc *client.DjinniClient, jobSlug string, message string, cvFileName string, cvContent []byte, extraFormData map[string]string) (string, error) {
	if jobSlug == "" {
		return "", errors.New("jobSlug cannot be empty")
	}

	targetURL := fmt.Sprintf("https://djinni.co/jobs/%s/?ref=for_me", jobSlug)
	if dc.Client.BaseURL != "" {
		targetURL = fmt.Sprintf("%s/jobs/%s/?ref=for_me", dc.Client.BaseURL, jobSlug)
	}

	formData := map[string]string{
		"apply":               "true",
		"message":             message,
		"msg_template_name":   "",
		"csrfmiddlewaretoken": dc.Config.CSRFToken,
	}
	for k, v := range extraFormData {
		formData[k] = v
	}

	req := dc.Client.R().
		EnableForceMultipart().
		SetFormData(formData).
		SetHeader("Referer", targetURL).
		SetHeader("X-CSRFToken", dc.Config.CSRFToken)

	if len(cvContent) > 0 {
		req.SetFileReader("cv_file", cvFileName, bytes.NewReader(cvContent))
	}

	resp, err := req.Post(targetURL)
	if err != nil {
		return "", fmt.Errorf("failed to apply to job: %w", err)
	}
	if !resp.IsSuccess() {
		return "", fmt.Errorf("unexpected status code applying to job: %d", resp.StatusCode)
	}

	// req/v3 follows redirects. Confirm redirect to "?applied=ok".
	finalURL := ""
	if resp.Response != nil && resp.Response.Request != nil && resp.Response.Request.URL != nil {
		finalURL = resp.Response.Request.URL.String()
	}
	// FRAGILE: Relying on redirect URLs containing "applied=ok" to confirm application success.
	// TODO: Replace with a more robust response validation or API verification.
	
	if !strings.Contains(finalURL, "applied=ok") && !strings.Contains(finalURL, "applied=1") && !strings.Contains(resp.String(), "b-application-status--success") {
		os.WriteFile("logs/failed_apply.html", resp.Bytes(), 0644); return "", fmt.Errorf("application redirection check failed: expected applied=ok in final URL %q", finalURL)
	}

	return "Application Success", nil
}
