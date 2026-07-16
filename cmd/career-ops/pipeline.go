package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
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

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func retryPendingApplications(ctx context.Context, dc *client.DjinniClient) {
	fmt.Println("🔄 Retrying pending applications...")
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
		
		fmt.Printf("📤 Retrying application for %s...\n", app.JobSlug)
		_, err := api.ApplyToJob(dc, app.JobSlug, app.Message, app.CVFileName, cvBytes, app.ExtraFormData)
		if err != nil {
			fmt.Printf("⚠️ Still failing for %s: %v\n", app.JobSlug, err)
			remaining = append(remaining, app)
		} else {
			fmt.Printf("✅ Success for %s!\n", app.JobSlug)
			successCount++
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

	notify.SendTelegramMessage(fmt.Sprintf("🔄 *Retry Complete*\nSuccessfully applied to %d out of %d pending jobs.", successCount, len(apps)))
}

func setupBotCommands(bot *notify.TelegramBot, dc *client.DjinniClient, ctx context.Context) {
	bot.AddCommand("/set_session", func(m *notify.TGMessage) {
		parts := strings.SplitN(m.Text, " ", 2)
		if len(parts) < 2 {
			notify.SendTelegramMessage("Usage: `/set_session <new_sessionid>`")
			return
		}
		newToken := strings.TrimSpace(parts[1])
		
		err := config.UpdateEnvFile(flagContextDir, "DJINNI_SESSIONID", newToken)
		if err != nil {
			notify.SendTelegramMessage(fmt.Sprintf("Failed to update token: %v", err))
			return
		}
		
		godotenv.Overload(filepath.Join(flagContextDir, ".env"))
		cfg, err := config.LoadConfig()
		if err != nil {
			notify.SendTelegramMessage(fmt.Sprintf("Failed to reload config: %v", err))
			return
		}
		
		dc.Config = cfg
		dc.Client.SetCommonCookies(nil)
		
		newDc := client.NewDjinniClient(cfg)
		dc.Client = newDc.Client

		notify.SendTelegramMessage("✅ Session ID updated successfully. Validating...")
		
		if api.CheckToken(dc) {
			notify.SendTelegramMessage("✅ Session ID is valid! Retrying pending applications...")
			go retryPendingApplications(ctx, dc)
		} else {
			notify.SendTelegramMessage("🚨 The new token appears to be invalid or expired. Please check and try again.")
		}
	})
}


var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage the autonomous job scan and apply pipeline",
}

var pipelineRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the full automated scan, evaluate, and apply cycle",
	RunE:  runPipelineRun,
}

var pipelineInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "Process unread messages in inbox and auto-reply",
	RunE:  runPipelineInbox,
}

var (
	flagThreshold float64
	flagDryRun    bool
	flagLimit     int
	flagDaemon    bool
)

type appliedJobInfo struct {
	Company string
	Title   string
	Score   float64
	DryRun  bool
}

func init() {
	pipelineRunCmd.Flags().Float64Var(&flagThreshold, "threshold", 3.5, "Score threshold to trigger auto-apply (0.0 to 5.0)")
	pipelineRunCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Scan and evaluate jobs, but do not send applications")
	pipelineRunCmd.Flags().IntVar(&flagLimit, "limit", 5, "Maximum number of applications to submit in this run")
	pipelineRunCmd.Flags().BoolVar(&flagDaemon, "daemon", false, "Run continuously in background, spreading up to 15 applications daily between 9 AM and 9 PM")

	pipelineInboxCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Generate replies but do not send them to recruiters")

	pipelineCmd.AddCommand(pipelineRunCmd)
	pipelineCmd.AddCommand(pipelineInboxCmd)
	rootCmd.AddCommand(pipelineCmd)
}

func runPipelineRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	go func() {
		s := <-sigChan
		fmt.Println("\n🛑  Interrupted by user. Exiting gracefully... (Press Ctrl+C again to force exit)")
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
		notify.SendTelegramMessage("🚨 Djinni sessionid cookie expired or invalid! Send `/set_session <your_sessionid>` to update it.")
	}

	// 2. Load Deduplicator
	fmt.Printf("📂  Loading deduplication history from %s...\n", flagContextDir)
	dedup, err := pipeline.LoadDedup(flagContextDir)
	if err != nil {
		return fmt.Errorf("failed to load deduplication history: %w", err)
	}

	// 3. Scan Djinni for relevant jobs
	fmt.Println("🔍  Scanning Djinni for new positions...")
	jobs, err := pipeline.ScanDjinni(flagContextDir, dc, dedup)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(jobs) == 0 {
		fmt.Println("✅  No new relevant jobs found.")
		return nil
	}

	fmt.Printf("🎯  Found %d new relevant job(s) to process.\n", len(jobs))
	appliedCount := 0
	skippedThreshold := 0
	skippedDedupe := 0
	pdfCount := 0
	errorCount := 0

	var appliedJobs []appliedJobInfo
	panicStop := &atomic.Bool{}

	for _, j := range jobs {
		if panicStop.Load() {
			fmt.Println("🛑  PanicStop triggered. Halting runPipelineRun loop.")
			break
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
			fmt.Printf("🛑  Daily application limit (%d) reached. Stopping.\n", flagLimit)
			break
		}

		applied, err := processJobItem(ctx, panicStop, cfg, bot, dc, engine, dedup, j, &skippedDedupe, &skippedThreshold, &errorCount, &pdfCount, &appliedJobs)
		if err != nil {
			fmt.Printf("⚠️   Error processing job %s: %v\n", j.Title, err)
			continue
		}
		if applied {
			appliedCount++
		}
	}

	// Send an aggregated summary report to Telegram to reduce spam
	var summary strings.Builder
	summary.WriteString("📊 *Career-Ops Run Summary*\n")
	summary.WriteString(fmt.Sprintf("🕒 Date: %s\n", time.Now().Format("2006-01-02 15:04")))
	summary.WriteString(fmt.Sprintf("🔎 Relevant scanned: %d\n", len(jobs)))
	summary.WriteString(fmt.Sprintf("✅ Applied: %d\n", appliedCount))
	summary.WriteString(fmt.Sprintf("⏭ Skipped (low score): %d\n", skippedThreshold))
	summary.WriteString(fmt.Sprintf("⏭ Skipped (already applied): %d\n", skippedDedupe))
	summary.WriteString(fmt.Sprintf("📄 PDFs Generated: %d\n", pdfCount))
	summary.WriteString(fmt.Sprintf("❌ Errors: %d\n\n", errorCount))

	if len(appliedJobs) > 0 {
		if flagDryRun {
			summary.WriteString("🚀 *Potential Applications (Dry-Run):*\n")
		} else {
			summary.WriteString("🚀 *Applied Positions:*\n")
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

func regenerateCoverLetter(ctx context.Context, cfg *config.Config, engine llm.Engine, details *api.JobFull, oldMsg, instruction string) (string, error) {
	provider, err := llm.NewProvider(cfg, engine)
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

func processJobItem(ctx context.Context, panicStop *atomic.Bool, cfg *config.Config, bot *notify.TelegramBot, dc *client.DjinniClient, engine llm.Engine, dedup *pipeline.Dedup, j extractor.JobSummary, skippedDedupe, skippedThreshold, errorCount, pdfCount *int, appliedJobs *[]appliedJobInfo) (bool, error) {
	fmt.Printf("\n%-66s\n", "⚡ Processing: "+j.Title)
	logDeep("PROCESS_JOB_ITEM", fmt.Sprintf("Fetching details for %s", j.Slug))

	if panicStop != nil && panicStop.Load() {
		return false, fmt.Errorf("panicStop triggered")
	}

	// Fetch full job details
	details, err := api.GetJobDetails(dc, j.Slug)
	if err != nil {
		*errorCount++
		logDeep("ERROR", fmt.Sprintf("GetJobDetails failed for %s: %v", j.Slug, err))
		return false, err
	}

	// Double check deduplication now that we have the exact company name
	if !dedup.IsNew(j.URL, details.Company, details.Title) {
		msg := fmt.Sprintf("Already applied/scanned a similar role at %s. Skipping.", details.Company)
		logDeep("DEDUP", msg)
		fmt.Printf("⏭   %s\n", msg)
		*skippedDedupe++
		return false, nil
	}

	// Evaluate the job
	logDeep("EVALUATE", fmt.Sprintf("Evaluating role at %s...", details.Company))
	fmt.Printf("🤖  Evaluating role at %s...\n", details.Company)
	
	// Add delay BEFORE evaluation to respect free LLM rate limits
	if engine == "freellmapi" {
		fmt.Println("⏳ Waiting 30 seconds to respect free LLM API rate limits...")
		time.Sleep(30 * time.Second)
	}

	res, err := pipeline.EvaluateJob(ctx, details.Description, cfg, engine, flagContextDir, "evaluation")
	if err != nil {
		*errorCount++
		logDeep("ERROR", fmt.Sprintf("EvaluateJob failed for %s: %v", details.Company, err))
		
		if engine == "freellmapi" {
			fmt.Println("⚠️ Free API Error encountered. Waiting 60 seconds for quota reset...")
			time.Sleep(60 * time.Second)
		}
		return false, err
	}

	// Save to scan-history.tsv only after successful evaluation, so we don't lose it on LLM crash
	pipeline.AppendToScanHistory(flagContextDir, j.URL, "Djinni Scan", j.Title, details.Company)

	logDeep("EVAL_RESULT", fmt.Sprintf("Score: %.1f/5 | Archetype: %s | Legitimacy: %s", res.Score, res.Archetype, res.Legitimacy))
	fmt.Printf("📊  Score: %.1f/5 | Archetype: %s | Legitimacy: %s\n", res.Score, res.Archetype, res.Legitimacy)

	// Trigger merge-tracker to merge evaluated status immediately
	runMergeTracker(flagContextDir)

	// Apply if score meets threshold
	if res.Score >= flagThreshold {
		logDeep("APPLY_START", fmt.Sprintf("Score %.1f >= %.1f. Proceeding to auto-apply.", res.Score, flagThreshold))
		fmt.Printf("🔥  High match (%.1f >= %.1f). Auto-applying!\n", res.Score, flagThreshold)
		reportAbsPath := filepath.Join(flagContextDir, res.ReportPath)

		if engine == "freellmapi" {
			fmt.Println("⏳ Waiting 20 seconds before generating CV to respect free LLM API rate limits...")
			time.Sleep(20 * time.Second)
		}
		
		// Generate tailored CV PDF
		logDeep("CV_GENERATE", fmt.Sprintf("Generating tailored CV PDF for %s", details.Company))
		fmt.Printf("📄 Generating tailored CV PDF for %s...\n", details.Company)
		cvBytes, err := covergen.GenerateCustomCV(ctx, cfg, engine, flagContextDir, j.URL, details.Company, details.Title, reportAbsPath)
		if err != nil {
			*errorCount++
			logDeep("ERROR", fmt.Sprintf("Custom CV generation failed for %s: %v", details.Company, err))
			return false, fmt.Errorf("custom CV generation failed: %w", err)
		}

		if engine == "freellmapi" {
			fmt.Println("⏳ Waiting 20 seconds before generating Cover Letter...")
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
			fmt.Printf("📝  Quiz detected: %d question(s) for quiz %s\n", len(details.QuizQuestions), details.QuizID)
			logDeep("QUIZ_DETECTED", fmt.Sprintf("%d question(s) for quiz %s", len(details.QuizQuestions), details.QuizID))

			if engine == "freellmapi" {
				fmt.Println("⏳ Waiting 20 seconds before answering quiz to respect free LLM API rate limits...")
				time.Sleep(20 * time.Second)
			}

			logDeep("QUIZ_ANSWERING", "Calling LLM to answer quiz questions")
			fmt.Printf("🤖  Answering %d quiz question(s)...\n", len(details.QuizQuestions))
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
			fmt.Printf("✅  Quiz answers ready (%d fields).\n", len(answered))
		}

		if flagDryRun {
			if extraFormData != nil {
				fmt.Printf("📝  Quiz answers would be submitted:\n")
				for k, v := range extraFormData {
					fmt.Printf("      %s = %s\n", k, v)
				}
			}
			msg := fmt.Sprintf("[DRY-RUN] Would apply to %s with generated custom CV PDF and message: %q", details.Company, introMsg)
			logDeep("APPLY_DRYRUN", msg)
			fmt.Printf("%s\n", msg)
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
					instruction, accept, retMsgID, err := pipeline.AskUserForApplyReview(ctx, bot, details.Company, details.Title, j.URL, res.Score, cvFileName, introMsg, j.Slug, msgID)
					msgID = retMsgID
					if err != nil {
						*errorCount++
						logDeep("ERROR", fmt.Sprintf("AskUserForApplyReview failed: %v", err))
						return false, err
					}

					if strings.HasPrefix(instruction, "edit:") {
						editMsg := strings.TrimPrefix(instruction, "edit:")
						fmt.Printf("🔄  Regenerating cover letter with instruction: %q\n", editMsg)
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
						fmt.Printf("🚫  Application to %s rejected by user.\n", details.Company)
						return false, nil
					}
					break
				}

				text := fmt.Sprintf(
					"📋 *Job Review Required*\n\n"+
						"*Company:* %s\n"+
						"*Role:* %s\n"+
						"*Score:* %.1f/5\n"+
						"*URL:* %s\n"+
						"*CV:* %s\n\n"+
						"*Cover Letter:*\n%s\n\n"+
						"Should I apply to this role?",
					details.Company, details.Title, res.Score, j.URL, cvFileName, introMsg,
				)

				logDeep("APPLY_SUBMIT", fmt.Sprintf("Submitting application to %s...", details.Company))
				fmt.Printf("📤  Submitting application to %s...\n", details.Company)
				
				// Submit application with the tailored CV PDF (and quiz answers if any)
				_, err = api.ApplyToJob(dc, j.Slug, introMsg, cvFileName, cvBytes, extraFormData)
				if err != nil {
					*errorCount++
					logDeep("ERROR", fmt.Sprintf("Application submission failed to %s: %v", details.Company, err))
					_ = notify.EditMessageText(msgID, text+"\n\n🔴 *Status:* Failed to apply (queued for retry): "+err.Error())
					
					app := pipeline.PendingApplication{
						JobSlug:       j.Slug,
						Message:       introMsg,
						CVFileName:    cvFileName,
						ExtraFormData: extraFormData,
					}
					pipeline.SavePendingApplication(flagContextDir, app, cvBytes)
					
					return false, fmt.Errorf("application submission failed (queued): %w", err)
				}

				_ = notify.EditMessageText(msgID, text+"\n\n🟢 *Status:* Application accepted and submitted.")
				logDeep("APPLY_SUCCESS", fmt.Sprintf("Successfully applied to %s", details.Company))
				*pdfCount++
				*appliedJobs = append(*appliedJobs, appliedJobInfo{
					Company: details.Company,
					Title:   details.Title,
					Score:   res.Score,
					DryRun:  false,
				})

				// Create applied TSV tracker entry so merge-tracker upgrades status to "Applied"
				createAppliedTrackerAddition(flagContextDir, res, details.Company, details.Title, providerName(cfg, engine))
				runMergeTracker(flagContextDir)
				return true, nil
			}
	} else {
		msg := fmt.Sprintf("Score (%.1f) below threshold (%.1f). Skipping apply.", res.Score, flagThreshold)
		logDeep("SKIP_LOW_SCORE", msg)
		fmt.Printf("⏭   %s\n", msg)
		*skippedThreshold++
		return false, nil
	}
}

type daemonStats struct {
	StartTime        time.Time
	ScansCount       int
	AppliedCount     int
	SkippedThreshold int
	SkippedDedupe    int
	ErrorCount       int
	PdfCount         int
	AppliedJobs      []appliedJobInfo
}

func runDaemonMode(ctx context.Context, cfg *config.Config, sigChan chan os.Signal) error {
	fmt.Println("🚀 Starting Career-Ops Pipeline in Daemon Mode (Debug/Continuous)...")
	logDeep("START", "Daemon mode started with deep logging enabled.")

	// ── Restore persisted session token ──────────────────────────────────────
	// /set_session saves the token to career-ops/.env (mounted as a Docker volume).
	// We reload it here so the bot remembers the token across container restarts.
	savedEnvPath := filepath.Join(flagContextDir, ".env")
	if err := godotenv.Overload(savedEnvPath); err == nil {
		fmt.Printf("🔑 Loaded persisted session from %s\n", savedEnvPath)
		// Rebuild config with the restored token
		if reloaded, err := config.LoadConfig(); err == nil {
			cfg = reloaded
		}
	} else {
		fmt.Printf("ℹ️  No persisted .env found at %s (will use environment vars)\n", savedEnvPath)
	}

	dc := client.NewDjinniClient(cfg)
	engine := llm.Engine(flagEngine)
	panicStop := &atomic.Bool{}

	bot := notify.NewTelegramBot()
	bot.Start()
	bot.StartStatusBoard()
	defer bot.Stop()
	setupBotCommands(bot, dc, ctx)

	// ── Initial silent token check ────────────────────────────────────────────
	// Validate token on startup WITHOUT notifying Telegram — it may be perfectly
	// valid (just restored from .env). Only alert if it's actually expired.
	fmt.Println("🔍 Validating session token on startup...")
	if api.CheckToken(dc) {
		fmt.Println("✅ Session token is valid. Starting pipeline.")
	} else {
		fmt.Println("🚨 Session token is invalid or expired. Waiting for /set_session.")
		notify.SendTelegramMessage("🚨 *Djinni session expired after restart!*\nPlease send your new session cookie:\n`/set_session <your_sessionid>`")
	}

	stats := daemonStats{StartTime: time.Now()}

	updateSummary := func() {
		var summary strings.Builder
		summary.WriteString("📊 *Daemon Mode Cumulative Summary*\n")
		summary.WriteString(fmt.Sprintf("🕒 Started: %s\n", stats.StartTime.Format("2006-01-02 15:04")))
		summary.WriteString(fmt.Sprintf("🔄 Scans: %d\n", stats.ScansCount))
		summary.WriteString(fmt.Sprintf("✅ Applied: %d\n", stats.AppliedCount))
		summary.WriteString(fmt.Sprintf("⏭ Skipped (low score): %d\n", stats.SkippedThreshold))
		summary.WriteString(fmt.Sprintf("⏭ Skipped (already applied): %d\n", stats.SkippedDedupe))
		summary.WriteString(fmt.Sprintf("📄 PDFs Generated: %d\n", stats.PdfCount))
		summary.WriteString(fmt.Sprintf("❌ Errors: %d\n\n", stats.ErrorCount))

		if len(stats.AppliedJobs) > 0 {
			summary.WriteString("🚀 *Applied Positions:*\n")
			for _, app := range stats.AppliedJobs {
				summary.WriteString(fmt.Sprintf("- %s — %s (Score: %.1f)\n", app.Company, app.Title, app.Score))
			}
		}
		bot.SetLastSummary(summary.String())
	}
	updateSummary()

	lastScanTime := time.Time{}
	scanInterval := 1 * time.Minute
	tokenInvalidNotified := false // track so we only send Telegram alert once per cycle

	for {
		if panicStop.Load() {
			fmt.Println("🛑  PanicStop triggered. Exiting daemon mode.")
			return nil
		}

		if !api.CheckToken(dc) {
			if !tokenInvalidNotified {
				notify.SendTelegramMessage("🚨 Djinni sessionid cookie expired or invalid! Waiting for update via `/set_session <your_sessionid>`.")
				tokenInvalidNotified = true
			}
			fmt.Println("🚨 Token invalid. Waiting 2 minutes...")
			time.Sleep(2 * time.Minute)
			continue
		}
		tokenInvalidNotified = false // reset when token becomes valid again

		now := time.Now()
		var scanTriggered bool
		if now.Sub(lastScanTime) >= scanInterval {
			lastScanTime = now
			scanTriggered = true
			stats.ScansCount++

			logDeep("SCAN_START", "Scanning Djinni for new positions...")
			dedup, err := pipeline.LoadDedup(flagContextDir)
			if err != nil {
				msg := fmt.Sprintf("Failed to load deduplication history: %v", err)
				logDeep("ERROR", msg)
				fmt.Printf("⚠️ %s. Retrying in 1 minute...\n", msg)
			} else {
				fmt.Println("🔍  Scanning Djinni for new positions...")
				jobs, err := pipeline.ScanDjinni(flagContextDir, dc, dedup)
				if err != nil {
					msg := fmt.Sprintf("Scan failed: %v", err)
					logDeep("ERROR", msg)
					fmt.Printf("⚠️ %s. Retrying in 1 minute...\n", msg)
				} else if len(jobs) > 0 {
					msg := fmt.Sprintf("Found %d relevant job(s) to process.", len(jobs))
					logDeep("SCAN_RESULT", msg)
					fmt.Printf("🎯 %s\n", msg)

					for _, j := range jobs {
						if panicStop.Load() {
							fmt.Println("🛑  PanicStop triggered. Halting job processing loop.")
							break
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
						logDeep("PROCESS_JOB", fmt.Sprintf("Starting evaluation for job: %s (%s)", j.Title, j.URL))

						skippedDedupe := 0
						skippedThreshold := 0
						errorCount := 0
						pdfCount := 0
						var appliedJobs []appliedJobInfo

						applied, err := processJobItem(ctx, panicStop, cfg, bot, dc, engine, dedup, j, &skippedDedupe, &skippedThreshold, &errorCount, &pdfCount, &appliedJobs)
						if err != nil {
							errMsg := fmt.Sprintf("Error processing job %s: %v", j.Title, err)
							logDeep("PROCESS_ERROR", errMsg)
							fmt.Printf("⚠️ %s\n", errMsg)
						}

						stats.SkippedDedupe += skippedDedupe
						stats.SkippedThreshold += skippedThreshold
						stats.ErrorCount += errorCount
						stats.PdfCount += pdfCount
						stats.AppliedJobs = append(stats.AppliedJobs, appliedJobs...)
						if applied {
							stats.AppliedCount++
						}
					}
				} else {
					logDeep("SCAN_RESULT", "No new relevant positions found.")
					fmt.Println("✅ No new relevant positions found.")
				}
			}
			updateSummary()
		}

		nextScan := lastScanTime.Add(scanInterval)
		sleepDur := time.Until(nextScan)
		if sleepDur <= 0 {
			sleepDur = 1 * time.Second
		}

		if scanTriggered {
			logDeep("SLEEP", fmt.Sprintf("Sleeping for %v before next scan.", sleepDur))
			fmt.Printf("💤 Sleeping for %v...\n", sleepDur)
		}

		select {
		case <-ctx.Done():
			logDeep("STOP", "Context cancelled, exiting daemon.")
			return ctx.Err()
		case s := <-sigChan:
			sigChan <- s
			logDeep("STOP", "Interrupted, exiting daemon.")
			return nil
		case update := <-bot.UpdateChan:
			if update.Message != nil && update.Message.Text != "" {
				text := update.Message.Text
				re := regexp.MustCompile(`https://djinni\.co/jobs/(\d+-[a-zA-Z0-9-]+)/?`)
				match := re.FindStringSubmatch(text)
				if len(match) > 1 {
					slug := match[1]
					url := match[0]

					notify.SendTelegramMessage(fmt.Sprintf("🔍 Processing manual job URL: %s", url))
					fmt.Printf("🔍 Processing manual job URL: %s\n", url)

					j := extractor.JobSummary{
						Slug:  slug,
						URL:   url,
						Title: "Manual Job",
					}

					skippedDedupe := 0
					skippedThreshold := 0
					errorCount := 0
					pdfCount := 0
					var appliedJobs []appliedJobInfo

					applied, err := processJobItem(ctx, panicStop, cfg, bot, dc, engine, &pipeline.Dedup{}, j, &skippedDedupe, &skippedThreshold, &errorCount, &pdfCount, &appliedJobs)
					if err != nil {
						notify.SendTelegramMessage(fmt.Sprintf("❌ Failed to process manual job: %v", err))
						stats.ErrorCount++
					} else {
						stats.SkippedDedupe += skippedDedupe
						stats.SkippedThreshold += skippedThreshold
						stats.ErrorCount += errorCount
						stats.PdfCount += pdfCount
						stats.AppliedJobs = append(stats.AppliedJobs, appliedJobs...)
						if applied {
							stats.AppliedCount++
						}
					}
					updateSummary()
				}
			}
		case <-time.After(sleepDur):
		}
	}
}

func runPipelineInbox(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	defer signal.Stop(sigChan)

	go func() {
		s := <-sigChan
		fmt.Println("\n🛑  Interrupted by user. Exiting gracefully... (Press Ctrl+C again to force exit)")
		signal.Stop(sigChan)
		sigChan <- s
	}()

	// 1. Load config
	_ = godotenv.Overload(filepath.Join(flagContextDir, ".env"))
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load Djinni config (session credentials required to read/reply inbox): %w", err)
	}

	dc := client.NewDjinniClient(cfg)
	engine := llm.Engine(flagEngine)

	bot := notify.NewTelegramBot()
	bot.Start()
	defer bot.Stop()
	setupBotCommands(bot, dc, ctx)

	if !api.CheckToken(dc) {
		notify.SendTelegramMessage("🚨 Djinni sessionid cookie expired or invalid! Send `/set_session <your_sessionid>` to update it.")
		return fmt.Errorf("invalid token, cannot process inbox")
	}

	panicStop := &atomic.Bool{}

	fmt.Println("📩  Scanning inbox for unread dialogue messages...")
	logs, err := pipeline.ProcessInbox(ctx, bot, panicStop, sigChan, cfg, engine, flagContextDir, dc, flagDryRun)
	if err != nil {
		return err
	}

	repliedCount := 0
	skippedCount := 0
	errorCount := 0
	var summary strings.Builder
	summary.WriteString("📩 *Recruiter Inbox Processed*\n")
	summary.WriteString(fmt.Sprintf("🕒 Date: %s\n\n", time.Now().Format("2006-01-02 15:04")))

	for _, logLine := range logs {
		fmt.Println(logLine)
		if strings.Contains(logLine, "Reply:") {
			repliedCount++
			summary.WriteString(fmt.Sprintf("💬 %s\n", logLine))
		} else if strings.Contains(logLine, "Skipped") {
			skippedCount++
		} else {
			errorCount++
			summary.WriteString(fmt.Sprintf("⚠️ %s\n", logLine))
		}
	}

	summary.WriteString(fmt.Sprintf("\n📊 *Summary:* Replied: %d | Skipped: %d | Errors: %d", repliedCount, skippedCount, errorCount))

	// Only send a TG message if we actually replied or had errors, avoiding empty check spam
	if repliedCount > 0 || errorCount > 0 {
		_ = notify.SendTelegramMessage(summary.String())
	}

	if bot != nil {
		bot.SetLastSummary(summary.String())
	}

	return nil
}

func runMergeTracker(contextDir string) {
	cmd := exec.Command("node", "merge-tracker.mjs")
	cmd.Dir = contextDir
	_ = cmd.Run()
}

func providerName(cfg *config.Config, engine llm.Engine) string {
	provider, err := llm.NewProvider(cfg, engine)
	if err != nil {
		return string(engine)
	}
	return provider.Name()
}

func createAppliedTrackerAddition(contextDir string, res *pipeline.EvalResult, company, role, toolLabel string) {
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
