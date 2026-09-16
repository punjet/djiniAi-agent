package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"database/sql"
	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/db"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/logger"
	"djinni-bot-go/internal/notify"
	"djinni-bot-go/internal/pipeline"
	"net/http"
)

func retryPendingApplications(ctx context.Context, dc *client.DjinniClient) {
	logger.Log.Info(" Retrying pending applications...")
	apps, err := pipeline.LoadPendingApplications(flagContextDir)
	if err != nil || len(apps) == 0 {
		return
	}

	successCount := 0
	var remaining []pipeline.PendingApplication

	for _, app := range apps {
		var cvBytes []byte
		if app.CVPath != "" {
			cvBytes, _ = os.ReadFile(app.CVPath)
		}

		logger.Log.Info(" Retrying application", "job_slug", app.JobSlug)
		_, err := api.ApplyToJob(dc, app.JobSlug, app.Message, app.CVFileName, cvBytes, app.ExtraFormData)
		if err != nil {
			logger.Log.Error("Still failing application", "job_slug", app.JobSlug, "error", err)
			remaining = append(remaining, app)
		} else {
			logger.Log.Info(" Success for application", "job_slug", app.JobSlug)
			successCount++
			// Extract job ID from JobSlug (first part before \-)
			jobID := ""
			parts := strings.Split(app.JobSlug, "-")
			if len(parts) > 0 {
				jobID = parts[0]
			}
			if jobID != "" {
				if err := pipeline.SaveAppliedJob(jobID); err != nil {
					logDeep("WARNING", fmt.Sprintf("Failed to save applied job ID %s: %v", jobID, err))
				}
			}
		}
	}

	pipeline.ClearPendingApplications(flagContextDir)
	for _, app := range remaining {
		var cvBytes []byte
		if app.CVPath != "" {
			cvBytes, _ = os.ReadFile(app.CVPath)
		}
		pipeline.SavePendingApplication(flagContextDir, app, cvBytes)
	}

	notify.SendTelegramMessage(fmt.Sprintf(" *Retry Complete*\nSuccessfully applied to %d out of %d pending jobs.", successCount, len(apps)))
}

var globalDB *sql.DB

func startHTTPServer(cfg *config.Config) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	database, err := db.InitDB(cfg)
	if err != nil || database == nil {
		logger.Log.Error("Database initialization warning/error or DB is nil", "error", err)
	} else {
		globalDB = database
		if err := db.MigrateFilesToDB(database, flagContextDir); err != nil {
			logger.Log.Error("Data auto-migration warning/error", "error", err)
		}
	}

	handlers := api.NewHandlers(database, nil, nil)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	go func() {
		logger.Log.Info(" Web UI server listening", "port", port)
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			logger.Log.Error("HTTP server error", "error", err)
		}
	}()
}

func regenerateCoverLetter(ctx context.Context, cfg *config.Config, engine llm.Engine, details *api.JobFull, oldMsg, instruction string) (string, error) {
	provider, err := llm.NewProvider(cfg, engine, "resume")
	if err != nil {
		return "", err
	}

	systemPrompt := `You are an expert technical resume writer. The user wants you to edit a previously generated Djinni cover letter/message hook.`
	userPrompt := fmt.Sprintf(`Job Details:
Company: %s
Role: %s
JD Content:
%s

Original Message:
%s

User Instruction:
%s

Rewrite the Original Message according to the User Instruction. Provide ONLY the rewritten text as your response. Do NOT include markdown wrappers.`, details.Company, details.Title, details.Description, oldMsg, instruction)

	response, err := provider.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("LLM cover letter regeneration failed: %w", err)
	}

	cleanText := strings.TrimSpace(response)
	cleanText = strings.TrimPrefix(cleanText, "```")
	cleanText = strings.TrimSuffix(cleanText, "```")
	cleanText = strings.TrimSpace(cleanText)
	return cleanText, nil
}

func logDeep(stage, message string) {
	logDir := filepath.Join(flagContextDir, "logs")
	os.MkdirAll(logDir, 0o755)
	logFile := filepath.Join(logDir, "deep_trace_"+time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		defer f.Close()
		logLine := fmt.Sprintf("[%s] [%s] %s\n", time.Now().Format(time.RFC3339), stage, message)
		f.WriteString(logLine)
	}
}

func runMergeTracker(contextDir string) {
	cmd := exec.Command("node", "merge-tracker.mjs")
	cmd.Dir = contextDir
	_ = cmd.Run()
}

func providerName(cfg *config.Config, engine llm.Engine) string {
	provider, err := llm.NewProvider(cfg, engine, "")
	if err != nil {
		return string(engine)
	}
	return provider.Name()
}

func createAppliedTrackerAddition(contextDir string, res *pipeline.EvalResult, company, role, toolLabel string, jobSlug ...string) {
	if globalDB != nil {
		slug := ""
		if len(jobSlug) > 0 {
			slug = jobSlug[0]
		}
		app := &db.Application{
			JobID:       slug,
			CompanyName: company,
			Status:      "applied",
		}
		if err := db.CreateApplication(globalDB, app); err != nil {
			logger.Log.Error("Database CreateApplication failed", "error", err)
		}
	}

	// Re-write a TSV entry to upgrade status to Applied in tracker additions
	trackerDir := filepath.Join(contextDir, "batch", "tracker-additions")
	_ = os.MkdirAll(trackerDir, 0o755)

	num := strings.Split(filepath.Base(res.ReportPath), "-")[0]
	today := time.Now().Format("2006-01-02")
	companySlug := strings.ToLower(company)
	companySlug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(companySlug, "-")

	trackerPath := filepath.Join(trackerDir, fmt.Sprintf("%s-%s.tsv", num, companySlug))
	fields := []string{
		num,
		today,
		company,
		role,
		"Applied",
		fmt.Sprintf("%.1f/5", res.Score),
		fmt.Sprintf("output/%s-%s-cover.pdf", companySlug, strings.ToLower(role)),
		fmt.Sprintf("[%s](reports/%s)", num, filepath.Base(res.ReportPath)),
		toolLabel + " auto-apply",
	}
	tsv := strings.Join(fields, "\t") + "\n"
	_ = os.WriteFile(trackerPath, []byte(tsv), 0o644)
}
