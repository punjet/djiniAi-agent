package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/covergen"
	"djinni-bot-go/internal/extractor"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/pipeline"
)

var pipelineTestApplyCmd = &cobra.Command{
	Use:   "test-apply",
	Short: "Run a single end-to-end flow with verbose trace output",
	RunE:  runPipelineTestApply,
}

var flagTestApplyDryRun bool

func init() {
	pipelineTestApplyCmd.Flags().BoolVar(&flagTestApplyDryRun, "dry-run", true, "Do not submit the application (default true)")
	pipelineCmd.AddCommand(pipelineTestApplyCmd)
}

func logTestApply(format string, args ...interface{}) {
	logDir := "logs"
	os.MkdirAll(logDir, 0o755)
	logFile := filepath.Join(logDir, "test-apply.log")

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format(time.RFC3339)
	line := fmt.Sprintf("[%s] %s\n", timestamp, msg)

	fmt.Print(line)

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil {
		f.WriteString(line)
		f.Close()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func runPipelineTestApply(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logTestApply("========================================")
	logTestApply("STARTING test-apply (dry-run: %v)", flagTestApplyDryRun)

	_ = godotenv.Overload(filepath.Join(flagContextDir, ".env"))
	cfg, err := config.LoadConfig()
	if err != nil {
		logTestApply("ERROR: Failed to load config: %v", err)
		return fmt.Errorf("config error: %w", err)
	}
	logTestApply("Config loaded. CSRFToken length: %d", len(cfg.CSRFToken))

	llm.GlobalTraceLogger = logTestApply
	dc := client.NewDjinniClient(cfg)

	// Add deep HTTP tracing
	dc.Client.OnBeforeRequest(func(c *req.Client, r *req.Request) error {
		urlStr := r.RawURL
		if r.URL != nil {
			urlStr = r.URL.String()
		}
		logTestApply("HTTP Request -> %s %s", r.Method, urlStr)
		logTestApply("Headers -> %v", r.Headers)
		return nil
	})

	dc.Client.OnAfterResponse(func(c *req.Client, r *req.Response) error {
		urlStr := ""
		if r.Request != nil {
			urlStr = r.Request.RawURL
			if r.Request.URL != nil {
				urlStr = r.Request.URL.String()
			}
			logTestApply("HTTP Response <- %s %s", r.Request.Method, urlStr)
		}
		logTestApply("Status -> %d", r.StatusCode)
		
		body := r.Bytes()
		logTestApply("Payload Size -> %d bytes", len(body))

		if r.Request != nil {
			logTestApply("Final URL (Redirect check) -> %s", urlStr)
			if strings.Contains(urlStr, "ref=for_me") || r.StatusCode >= 400 {
				logTestApply("Response Body Snippet -> %s", string(body[:min(1000, len(body))]))
			}
		}
		return nil
	})

	dedup, err := pipeline.LoadDedup(flagContextDir)
	if err != nil {
		logTestApply("Failed to load deduplication history: %v", err)
	}

	logTestApply("Scanning Djinni for a test job...")
	jobs, err := pipeline.ScanDjinni(flagContextDir, dc, dedup)

	var testJob extractor.JobSummary
	var details *api.JobFull

	if err == nil && len(jobs) > 0 {
		testJob = jobs[0]
		logTestApply("Selected job from scan: %s (%s)", testJob.Title, testJob.URL)
		logTestApply("Fetching details...")
		details, err = api.GetJobDetails(dc, testJob.Slug)
		if err != nil {
			logTestApply("Failed to get job details: %v", err)
		}
	}

	if details == nil {
		logTestApply("No jobs found or scan error. Using dummy job.")
		testJob = extractor.JobSummary{
			Slug:    "test-slug-1234",
			URL:     "https://djinni.co/jobs/test-slug-1234",
			Title:   "Test AI Developer",
		}
		details = &api.JobFull{
			ID:           "1234",
			Slug:         "test-slug-1234",
			Title:        "Test AI Developer",
			Company:      "Dummy Test Company",
			Description:  "We are looking for an AI Developer to work on agents, automation, and LLMs using Golang.",
			Requirements: "Go, LLM, Agents",
			URL:          "https://djinni.co/jobs/test-slug-1234",
		}
	}

	engine := llm.Engine(flagEngine)
	logTestApply("Evaluating job description with %s...", engine)

	evalStart := time.Now()
	res, err := pipeline.EvaluateJob(ctx, details.Description, cfg, engine, flagContextDir)
	evalLatency := time.Since(evalStart)

	if err != nil {
		logTestApply("EvaluateJob error: %v", err)
		return fmt.Errorf("evaluation failed: %w", err)
	}

	logTestApply("Evaluation Latency -> %v", evalLatency)
	logTestApply("Prompt/JD length approx -> %d chars", len(details.Description))
	logTestApply("Score Parsed -> %.1f", res.Score)
	logTestApply("Archetype -> %s", res.Archetype)
	logTestApply("Full LLM Response -> %s", res.FullText)

	logTestApply("Generating Custom CV...")
	cvBytes, err := covergen.GenerateCustomCV(ctx, cfg, engine, flagContextDir, testJob.URL, details.Company, details.Title, filepath.Join(flagContextDir, res.ReportPath))
	if err != nil {
		logTestApply("CV Generation error: %v", err)
		return fmt.Errorf("cv gen failed: %w", err)
	}
	logTestApply("CV PDF Size -> %d bytes", len(cvBytes))
	logTestApply("Playwright render triggered for CV HTML generation.")

	logTestApply("Generating Cover Letter...")
	_, introMsg, err := covergen.GenerateCoverLetter(ctx, cfg, engine, flagContextDir, details.Company, details.Title, details.Description)
	if err != nil {
		logTestApply("Cover letter generation error: %v", err)
		return fmt.Errorf("cover letter gen failed: %w", err)
	}
	logTestApply("Cover Letter message -> %q", introMsg)

	// Handle recruiter quiz (if present)
	var extraFormData map[string]string
	if details.QuizID != "" && len(details.QuizQuestions) > 0 {
		logTestApply("QUIZ DETECTED: %d question(s) for quiz %s", len(details.QuizQuestions), details.QuizID)
		for _, q := range details.QuizQuestions {
			logTestApply("  Question [%s]: %s", q.Name, q.Text)
		}

		if engine == "freellmapi" {
			logTestApply("⏳ Waiting 20 seconds before answering quiz to respect free LLM API rate limits...")
			time.Sleep(20 * time.Second)
		}

		logTestApply("Answering quiz questions with LLM...")
		answered, err := covergen.AnswerQuizQuestions(ctx, cfg, engine, flagContextDir, details.QuizQuestions, details.Description, details.Company, details.Title)
		if err != nil {
			logTestApply("Quiz answering failed: %v", err)
			return fmt.Errorf("quiz answering failed: %w", err)
		}

		extraFormData = make(map[string]string, len(answered)+1)
		extraFormData["quiz_id"] = details.QuizID
		for _, q := range answered {
			extraFormData[q.Name] = q.Answer
			logTestApply("  Answer [%s]: %s", q.Name, q.Answer)
		}
		logTestApply("Quiz answers prepared for submission.")
	}

	if flagTestApplyDryRun {
		logTestApply("DRY RUN: Skipping real application submission.")
	} else {
		logTestApply("SUBMITTING APPLICATION...")
		logTestApply("Submission Form Fields:")
		logTestApply("  - apply: true")
		logTestApply("  - message: (Cover Letter msg)")
		logTestApply("  - msg_template_name: (empty)")
		logTestApply("  - csrfmiddlewaretoken: %s", cfg.CSRFToken)
		logTestApply("  - cv_file: CV-Kyrylo-Kirov-%s.pdf (size %d)", details.Company, len(cvBytes))
		if extraFormData != nil {
			logTestApply("  - quiz_id: %s", extraFormData["quiz_id"])
			for k, v := range extraFormData {
				if k != "quiz_id" {
					logTestApply("  - %s: %s", k, v)
				}
			}
		}
		successMsg, err := api.ApplyToJob(dc, testJob.Slug, introMsg, fmt.Sprintf("CV-Kyrylo-Kirov-%s.pdf", details.Company), cvBytes, extraFormData)
		if err != nil {
			logTestApply("Application failed: %v", err)
			return fmt.Errorf("application failed: %w", err)
		}
		logTestApply("Application SUCCESS: %s", successMsg)
	}

	logTestApply("test-apply finished successfully.")
	return nil
}
