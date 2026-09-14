package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/covergen"
	"djinni-bot-go/internal/extractor"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/logger"
	"djinni-bot-go/internal/notify"
	"djinni-bot-go/internal/pipeline"
	"djinni-bot-go/internal/trace"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

type appliedJobInfo struct {
	Company string
	Title   string
	Score   float64
	DryRun  bool
}

func runPipelineRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	ctx = trace.WithTraceID(ctx, "")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	go func() {
		s := <-sigChan
		logger.Log.Info("\n  Interrupted by user. Exiting gracefully... (Press Ctrl+C again to force exit)")
		signal.Stop(sigChan)
		sigChan <- s
	}()

	// 1. Load config
	_ = godotenv.Overload(filepath.Join(flagContextDir, ".env"))
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load Djinni config (session credentials required for scan/apply): %w", err)
	}

	// Init logger early so all subsystems (api, covergen, etc.) have a non-nil logger
	logger.InitLogger(flagContextDir)

	startHTTPServer(cfg)

	if flagDaemon {
		return runDaemonMode(ctx, cfg, sigChan)
	}

	dc := client.NewDjinniClient(cfg)
	engine := llm.Engine(flagEngine)

	bot := notify.NewTelegramBot()
	bot.Start()
	defer bot.Stop()
	setupBotCommands(bot, dc, ctx)

	if !api.CheckToken(dc) {
		notify.SendTelegramMessage(" Djinni sessionid cookie expired or invalid! Send `/set_session <your_sessionid>` to update it.")
	}

	// 2. Load Deduplicator
	logger.Log.Info("  Loading deduplication history", "dir", flagContextDir)
	dedup, err := pipeline.LoadDedup(flagContextDir)
	if err != nil {
		return fmt.Errorf("failed to load deduplication history: %w", err)
	}

	// 3. Scan Djinni for relevant jobs
	logger.Log.Info("  Scanning Djinni for new positions...")
	jobs, err := pipeline.ScanDjinni(flagContextDir, dc, dedup)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(jobs) == 0 {
		logger.Log.Info("  No new relevant jobs found.")
		return nil
	}

	logger.Log.Info("  Found new relevant job(s) to process", "count", len(jobs))
	appliedCount := 0
	skippedThreshold := 0
	skippedDedupe := 0
	pdfCount := 0
	errorCount := 0

	var appliedJobs []appliedJobInfo

	for _, j := range jobs {
		select {
		case <-ctx.Done():
			logger.Log.Info(" Context cancelled. Halting runPipelineRun loop.")
			return ctx.Err()
		default:
		}

		interrupted := false
		select {
		case s := <-sigChan:
			sigChan <- s
			interrupted = true
		default:
		}
		if interrupted {
			break
		}

		if appliedCount >= flagLimit {
			logger.Log.Info("  Daily application limit reached. Stopping.", "limit", flagLimit)
			break
		}

		applied, err := processJobItem(ctx, cfg, bot, dc, engine, dedup, j, &skippedDedupe, &skippedThreshold, &errorCount, &pdfCount, &appliedJobs)
		if err != nil {
			logger.Log.Error("Error processing job", "title", j.Title, "error", err)
			continue
		}
		if applied {
			appliedCount++
		}
	}

	// Send an aggregated summary report to Telegram to reduce spam
	var summary strings.Builder
	summary.WriteString(" *Career-Ops Run Summary*\n")
	summary.WriteString(fmt.Sprintf(" Date: %s\n", time.Now().Format("2006-01-02 15:04")))
	summary.WriteString(fmt.Sprintf(" Relevant scanned: %d\n", len(jobs)))
	summary.WriteString(fmt.Sprintf(" Applied: %d\n", appliedCount))
	summary.WriteString(fmt.Sprintf(" Skipped (low score): %d\n", skippedThreshold))
	summary.WriteString(fmt.Sprintf(" Skipped (already applied): %d\n", skippedDedupe))
	summary.WriteString(fmt.Sprintf(" PDFs Generated: %d\n", pdfCount))
	summary.WriteString(fmt.Sprintf("❌ Errors: %d\n\n", errorCount))

	if len(appliedJobs) > 0 {
		if flagDryRun {
			summary.WriteString(" *Potential Applications (Dry-Run):*\n")
		} else {
			summary.WriteString(" *Applied Positions:*\n")
		}
		for _, app := range appliedJobs {
			summary.WriteString(fmt.Sprintf("- %s — %s (Score: %.1f)\n", app.Company, app.Title, app.Score))
		}
	}

	_ = notify.SendTelegramMessage(summary.String())

	if bot != nil {
		bot.SetLastSummary(summary.String())
	}

	return nil
}

func processJobItem(ctx context.Context, cfg *config.Config, bot *notify.TelegramBot, dc *client.DjinniClient, engine llm.Engine, dedup *pipeline.Dedup, j extractor.JobSummary, skippedDedupe, skippedThreshold, errorCount, pdfCount *int, appliedJobs *[]appliedJobInfo) (bool, error) {
	jobLogger := logger.FromContext(ctx).With("job_slug", j.Slug)
	ctx = logger.WithContext(ctx, jobLogger)
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// Load applied jobs registry
	appliedMap, err := pipeline.LoadAppliedJobs()
	if err != nil {
		logDeep("ERROR", fmt.Sprintf("Failed to load applied jobs registry: %v", err))
	} else {
		if _, ok := appliedMap[j.ID]; ok {
			msg := fmt.Sprintf("Skipping already applied job ID %s (registry)", j.ID)
			logDeep("REGISTRY_SKIP", msg)
			logger.FromContext(ctx).Info("Skipping already applied job ID (registry)", "job_id", j.ID)
			return false, nil
		}
		if _, ok := appliedMap[j.Slug]; ok {
			msg := fmt.Sprintf("Skipping already applied job %s (registry)", j.Slug)
			logDeep("REGISTRY_SKIP", msg)
			logger.FromContext(ctx).Info("Skipping already applied job (registry)", "job_slug", j.Slug)
			return false, nil
		}
	}
	logger.FromContext(ctx).Info("Processing job", "title", j.Title)
	logDeep("PROCESS_JOB_ITEM", fmt.Sprintf("Fetching details for %s", j.Slug))

	// Fetch full job details
	details, err := api.GetJobDetails(dc, j.Slug)
	if err != nil {
		*errorCount++
		logDeep("ERROR", fmt.Sprintf("GetJobDetails failed for %s: %v", j.Slug, err))
		return false, err
	}
	// HTML skip check
	if details.AlreadyApplied {
		msg := fmt.Sprintf("Already applied to %s (HTML snippet detected). Skipping.", details.Title)
		logDeep("HTML_SKIP", msg)
		logger.FromContext(ctx).Info("Already applied (HTML snippet detected). Skipping.")
		if err := pipeline.SaveAppliedJob(j.ID); err != nil {
			logDeep("WARNING", fmt.Sprintf("Failed to save applied job ID %s: %v", j.ID, err))
		}
		return false, nil
	}

	// Double check deduplication now that we have the exact company name
	if !dedup.IsNew(j.URL, details.Company, details.Title) {
		msg := fmt.Sprintf("Already applied/scanned a similar role at %s. Skipping.", details.Company)
		logDeep("DEDUP", msg)
		logger.FromContext(ctx).Info("Already applied/scanned a similar role at company. Skipping.", "company", details.Company)
		*skippedDedupe++
		return false, nil
	}

	// Evaluate the job
	logDeep("EVALUATE", fmt.Sprintf("Evaluating role at %s...", details.Company))
	logger.FromContext(ctx).Info("Evaluating role", "company", details.Company)

	// Add delay BEFORE evaluation to respect free LLM rate limits
	if engine == "freellmapi" {
		logger.FromContext(ctx).Info(" Waiting 30 seconds to respect free LLM API rate limits...")
		time.Sleep(30 * time.Second)
	}

	res, err := pipeline.EvaluateJob(ctx, details.Description, cfg, engine, flagContextDir, "evaluation")
	if err != nil {
		*errorCount++
		logDeep("ERROR", fmt.Sprintf("EvaluateJob failed for %s: %v", details.Company, err))

		if engine == "freellmapi" {
			logger.FromContext(ctx).Info(" Free API Error encountered. Waiting 60 seconds for quota reset...")
			time.Sleep(60 * time.Second)
		}
		return false, err
	}

	// Save to scan-history.tsv only after successful evaluation, so we don't lose it on LLM crash
	pipeline.AppendToScanHistory(flagContextDir, j.URL, "Djinni Scan", j.Title, details.Company)

	reportAbsPath := filepath.Join(flagContextDir, res.ReportPath)
	if f, err := os.OpenFile(reportAbsPath, os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		if _, err := f.WriteString(fmt.Sprintf("\n\n**Job ID:** %s\n**URL:** %s\n", j.ID, j.URL)); err != nil {
			logDeep("WARNING", fmt.Sprintf("Failed to append Job ID/URL to report %s: %v", res.ReportPath, err))
		}
		f.Close()
	} else {
		logDeep("WARNING", fmt.Sprintf("Failed to open report for appending %s: %v", res.ReportPath, err))
	}

	logDeep("EVAL_RESULT", fmt.Sprintf("Score: %.1f/5 | Archetype: %s | Legitimacy: %s", res.Score, res.Archetype, res.Legitimacy))
	logger.FromContext(ctx).Info("Evaluated role", "score", res.Score, "archetype", res.Archetype, "legitimacy", res.Legitimacy)

	// Trigger merge-tracker to merge evaluated status immediately
	runMergeTracker(flagContextDir)

	// Apply if score meets threshold
	if res.Score >= flagThreshold {
		logDeep("APPLY_START", fmt.Sprintf("Score %.1f >= %.1f. Proceeding to auto-apply.", res.Score, flagThreshold))
		logger.FromContext(ctx).Info("High match. Auto-applying!", "score", res.Score, "threshold", flagThreshold)

		if engine == "freellmapi" {
			logger.FromContext(ctx).Info(" Waiting 20 seconds before generating CV to respect free LLM API rate limits...")
			time.Sleep(20 * time.Second)
		}

		// Generate tailored CV PDF
		logDeep("CV_GENERATE", fmt.Sprintf("Generating tailored CV PDF for %s", details.Company))
		logger.FromContext(ctx).Info("Generating tailored CV PDF", "company", details.Company)
		cvBytes, err := covergen.GenerateCustomCV(ctx, cfg, engine, flagContextDir, j.URL, details.Company, details.Title, reportAbsPath, details.Description)
		if err != nil {
			*errorCount++
			logDeep("ERROR", fmt.Sprintf("Custom CV generation failed for %s: %v", details.Company, err))
			return false, fmt.Errorf("custom CV generation failed: %w", err)
		}

		if engine == "freellmapi" {
			logger.FromContext(ctx).Info(" Waiting 20 seconds before generating Cover Letter...")
			time.Sleep(20 * time.Second)
		}

		// Generate Cover Letter & Djinni Message
		logDeep("COVER_LETTER_GENERATE", fmt.Sprintf("Generating Cover Letter for %s", details.Company))
		_, introMsg, err := covergen.GenerateCoverLetter(ctx, cfg, engine, flagContextDir, details.Company, details.Title, details.Description)
		if err != nil {
			*errorCount++
			logDeep("ERROR", fmt.Sprintf("Cover letter generation failed for %s: %v", details.Company, err))
			return false, fmt.Errorf("cover letter generation failed: %w", err)
		}

		// Handle recruiter quiz (if present)
		var extraFormData map[string]string
		if details.QuizID != "" && len(details.QuizQuestions) > 0 {
			logger.FromContext(ctx).Info("Quiz detected", "questions_count", len(details.QuizQuestions), "quiz_id", details.QuizID)
			logDeep("QUIZ_DETECTED", fmt.Sprintf("%d question(s) for quiz %s", len(details.QuizQuestions), details.QuizID))

			if engine == "freellmapi" {
				logger.FromContext(ctx).Info(" Waiting 20 seconds before answering quiz to respect free LLM API rate limits...")
				time.Sleep(20 * time.Second)
			}

			logDeep("QUIZ_ANSWERING", "Calling LLM to answer quiz questions")
			logger.FromContext(ctx).Info("Answering quiz questions...", "count", len(details.QuizQuestions))
			answered, err := covergen.AnswerQuizQuestions(ctx, cfg, engine, flagContextDir, details.QuizQuestions, details.Description, details.Company, details.Title)
			if err != nil {
				*errorCount++
				logDeep("ERROR", fmt.Sprintf("Quiz answering failed for %s: %v", details.Company, err))
				return false, fmt.Errorf("quiz answering failed: %w", err)
			}

			extraFormData = make(map[string]string, len(answered)+1)
			extraFormData["quiz_id"] = details.QuizID
			for _, q := range answered {
				extraFormData[q.Name] = q.Answer
				logDeep("QUIZ_ANSWER", fmt.Sprintf("[%s] %s", q.Name, q.Answer))
			}
			logger.FromContext(ctx).Info("Quiz answers ready", "fields_count", len(answered))
		}

		if flagDryRun {
			if extraFormData != nil {
				logger.FromContext(ctx).Info("Quiz answers would be submitted:")
				for k, v := range extraFormData {
					logger.FromContext(ctx).Info("Quiz form field", "key", k, "value", v)
				}
			}
			msg := fmt.Sprintf("[DRY-RUN] Would apply to %s with generated custom CV PDF and message: %q", details.Company, introMsg)
			logDeep("APPLY_DRYRUN", msg)
			logger.FromContext(ctx).Info("[DRY-RUN] Would apply to company with generated custom CV PDF", "company", details.Company, "message", introMsg)
			*pdfCount++
			*appliedJobs = append(*appliedJobs, appliedJobInfo{
				Company: details.Company,
				Title:   details.Title,
				Score:   res.Score,
				DryRun:  true,
			})
			return true, nil
		} else {
			cvFileName := fmt.Sprintf("CV-Kyrylo-Kirov-%s.pdf", details.Company)

			_, errDoc := notify.SendDocument(cvFileName, cvBytes, fmt.Sprintf("CV tailored for %s", details.Company))
			if errDoc != nil {
				logDeep("WARNING", fmt.Sprintf("Failed to send CV document to Telegram: %v", errDoc))
			}

			var msgID int64

			for {
				instruction, accept, retMsgID, err := pipeline.AskUserForApplyReview(ctx, bot, details.Company, details.Title, j.URL, res.Summary, res.Score, cvFileName, introMsg, j.Slug, msgID)
				msgID = retMsgID
				if err != nil {
					*errorCount++
					logDeep("ERROR", fmt.Sprintf("AskUserForApplyReview failed: %v", err))
					return false, err
				}

				if strings.HasPrefix(instruction, "edit:") {
					editMsg := strings.TrimPrefix(instruction, "edit:")
					logger.FromContext(ctx).Info("Regenerating cover letter with instruction", "instruction", editMsg)
					newMsg, err := regenerateCoverLetter(ctx, cfg, engine, details, introMsg, editMsg)
					if err != nil {
						*errorCount++
						logDeep("ERROR", fmt.Sprintf("regenerateCoverLetter failed: %v", err))
						return false, err
					}
					introMsg = newMsg
					continue
				}

				if !accept {
					logger.FromContext(ctx).Info("Application rejected by user", "company", details.Company)
					return false, nil
				}
				break
			}

			logDeep("APPLY_SUBMIT", fmt.Sprintf("Submitting application to %s...", details.Company))
			logger.FromContext(ctx).Info("Submitting application to company...", "company", details.Company)

			// Submit application with the tailored CV PDF (and quiz answers if any)
			_, err = api.ApplyToJob(dc, j.Slug, introMsg, cvFileName, cvBytes, extraFormData)
			if err != nil {
				*errorCount++
				logDeep("ERROR", fmt.Sprintf("Application submission failed to %s: %v", details.Company, err))
				statusBlock := &notify.InputRichBlockParagraph{
					Type: "paragraph",
					Text: []interface{}{
						"\n\n ",
						notify.RichTextBold{Type: "bold", Text: "Status:"},
						" Failed to apply (queued for retry): " + err.Error(),
					},
				}
				richMsg := pipeline.BuildApplyReviewRichMessage(details.Company, details.Title, j.URL, res.Summary, res.Score, cvFileName, introMsg, statusBlock)
				_ = notify.EditRichMessageText(msgID, richMsg)

				app := pipeline.PendingApplication{
					JobSlug:       j.Slug,
					Message:       introMsg,
					CVFileName:    cvFileName,
					ExtraFormData: extraFormData,
				}
				pipeline.SavePendingApplication(flagContextDir, app, cvBytes)

				return false, fmt.Errorf("application submission failed (queued): %w", err)
			}

			statusBlock := &notify.InputRichBlockParagraph{
				Type: "paragraph",
				Text: []interface{}{
					"\n\n ",
					notify.RichTextBold{Type: "bold", Text: "Status:"},
					" Application accepted and submitted.",
				},
			}
			richMsg := pipeline.BuildApplyReviewRichMessage(details.Company, details.Title, j.URL, res.Summary, res.Score, cvFileName, introMsg, statusBlock)
			_ = notify.EditRichMessageText(msgID, richMsg)
			logDeep("APPLY_SUCCESS", fmt.Sprintf("Successfully applied to %s", details.Company))
			*pdfCount++
			*appliedJobs = append(*appliedJobs, appliedJobInfo{
				Company: details.Company,
				Title:   details.Title,
				Score:   res.Score,
				DryRun:  false,
			})
			// Persist job ID to applied jobs registry
			if err := pipeline.SaveAppliedJob(j.ID); err != nil {
				logDeep("WARNING", fmt.Sprintf("Failed to save applied job ID %s: %v", j.ID, err))
			}

			// Create applied TSV tracker entry so merge-tracker upgrades status to "Applied"
			createAppliedTrackerAddition(flagContextDir, res, details.Company, details.Title, providerName(cfg, engine), j.Slug)
			runMergeTracker(flagContextDir)
			return true, nil
		}
	} else {
		msg := fmt.Sprintf("Score (%.1f) below threshold (%.1f). Skipping apply.", res.Score, flagThreshold)
		logDeep("SKIP_LOW_SCORE", msg)
		logger.FromContext(ctx).Info("Score below threshold. Skipping apply.", "score", res.Score, "threshold", flagThreshold)
		*skippedThreshold++
		return false, nil
	}
}
