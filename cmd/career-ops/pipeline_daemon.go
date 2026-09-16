package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/extractor"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/logger"
	"djinni-bot-go/internal/notify"
	"djinni-bot-go/internal/pipeline"
	"djinni-bot-go/internal/trace"

	"github.com/joho/godotenv"
)

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
	logger.Log.Info(" Starting Career-Ops Pipeline in Daemon Mode (Debug/Continuous)...")
	logDeep("START", "Daemon mode started with deep logging enabled.")

	// ── Restore persisted session token ──────────────────────────────────────
	// /set_session saves the token to career-ops/.env (mounted as a Docker volume).
	// We reload it here so the bot remembers the token across container restarts.
	savedEnvPath := filepath.Join(flagContextDir, ".env")
	if err := godotenv.Overload(savedEnvPath); err == nil {
		logger.Log.Info(" Loaded persisted session", "path", savedEnvPath)
		// Rebuild config with the restored token
		if reloaded, err := config.LoadConfig(); err == nil {
			cfg = reloaded
		}
	} else {
		logger.Log.Info("  No persisted .env found (will use environment vars)", "path", savedEnvPath)
	}

	dc := client.NewDjinniClient(cfg)
	engine := llm.Engine(flagEngine)

	bot := notify.NewTelegramBot()
	bot.Start()
	bot.StartStatusBoard()
	defer bot.Stop()
	setupBotCommands(bot, dc, ctx)

	updateChan := bot.SubscribeUpdates()
	defer bot.UnsubscribeUpdates(updateChan)

	// ── Initial silent token check ────────────────────────────────────────────
	// Validate token on startup WITHOUT notifying Telegram — it may be perfectly
	// valid (just restored from .env). Only alert if it's actually expired.
	logger.Log.Info(" Validating session token on startup...")
	if api.CheckToken(dc) {
		logger.Log.Info(" Session token is valid. Starting pipeline.")
	} else {
		logger.Log.Info(" Session token is invalid or expired. Waiting for /set_session.")
		notify.SendTelegramMessage(" *Djinni session expired after restart!*\nPlease send your new session cookie:\n`/set_session <your_sessionid>`")
	}

	stats := daemonStats{StartTime: time.Now()}

	updateSummary := func() {
		var summary strings.Builder
		summary.WriteString(" *Daemon Mode Cumulative Summary*\n")
		summary.WriteString(fmt.Sprintf(" Started: %s\n", stats.StartTime.Format("2006-01-02 15:04")))
		summary.WriteString(fmt.Sprintf(" Scans: %d\n", stats.ScansCount))
		summary.WriteString(fmt.Sprintf(" Applied: %d\n", stats.AppliedCount))
		summary.WriteString(fmt.Sprintf(" Skipped (low score): %d\n", stats.SkippedThreshold))
		summary.WriteString(fmt.Sprintf(" Skipped (already applied): %d\n", stats.SkippedDedupe))
		summary.WriteString(fmt.Sprintf(" PDFs Generated: %d\n", stats.PdfCount))
		summary.WriteString(fmt.Sprintf("❌ Errors: %d\n\n", stats.ErrorCount))

		if len(stats.AppliedJobs) > 0 {
			summary.WriteString(" *Applied Positions:*\n")
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
		loopCtx := trace.WithTraceID(ctx, "")

		select {
		case <-loopCtx.Done():
			logger.Log.Info(" Context cancelled. Exiting daemon mode.")
			return loopCtx.Err()
		default:
		}

		if !api.CheckToken(dc) {
			if !tokenInvalidNotified {
				notify.SendTelegramMessage(" Djinni sessionid cookie expired or invalid! Waiting for update via `/set_session <your_sessionid>`.")
				tokenInvalidNotified = true
			}
			logger.Log.Info(" Token invalid. Waiting 2 minutes...")
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

			logDeep("INBOX_START", "Scanning Djinni inbox for unread messages...")
			logger.Log.Info("  Scanning inbox for unread dialogue messages...")
			inboxLogs, err := pipeline.ProcessInbox(loopCtx, bot, sigChan, cfg, engine, flagContextDir, dc, flagDryRun)
			if err != nil {
				logDeep("ERROR", fmt.Sprintf("Inbox processing failed: %v", err))
				logger.Log.Error("Inbox processing failed", "error", err)
			} else {
				for _, l := range inboxLogs {
					logger.Log.Info(fmt.Sprint("  ", l))
				}
			}

			logDeep("SCAN_START", "Scanning Djinni for new positions...")
			dedup, err := pipeline.LoadDedup(flagContextDir)
			if err != nil {
				msg := fmt.Sprintf("Failed to load deduplication history: %v", err)
				logDeep("ERROR", msg)
				logger.Log.Info(" Retrying load deduplication history in 1 minute...", "error", err)
			} else {
				logger.Log.Info("  Scanning Djinni for new positions...")
				jobs, err := pipeline.ScanDjinni(flagContextDir, dc, dedup)
				if err != nil {
					msg := fmt.Sprintf("Scan failed: %v", err)
					logDeep("ERROR", msg)
					logger.Log.Info(" Scan failed. Retrying in 1 minute...", "error", err)
				} else if len(jobs) > 0 {
					msg := fmt.Sprintf("Found %d relevant job(s) to process.", len(jobs))
					logDeep("SCAN_RESULT", msg)
					logger.Log.Info(" Found relevant job(s) to process", "count", len(jobs))

					for _, j := range jobs {
						select {
						case <-loopCtx.Done():
							logger.Log.Info(" Context cancelled. Halting job processing loop.")
							return loopCtx.Err()
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
						logDeep("PROCESS_JOB", fmt.Sprintf("Starting evaluation for job: %s (%s)", j.Title, j.URL))

						skippedDedupe := 0
						skippedThreshold := 0
						errorCount := 0
						pdfCount := 0
						var appliedJobs []appliedJobInfo

						applied, err := processJobItem(loopCtx, cfg, bot, dc, engine, dedup, j, &skippedDedupe, &skippedThreshold, &errorCount, &pdfCount, &appliedJobs)
						if err != nil {
							errMsg := fmt.Sprintf("Error processing job %s: %v", j.Title, err)
							logDeep("PROCESS_ERROR", errMsg)
							logger.Log.Info(" Error processing job", "title", j.Title, "error", err)
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
					logger.Log.Info(" No new relevant positions found.")
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
			logger.Log.Info(" Sleeping before next scan...", "duration", sleepDur)
		}

		select {
		case <-ctx.Done():
			logDeep("STOP", "Context cancelled, exiting daemon.")
			return ctx.Err()
		case s := <-sigChan:
			sigChan <- s
			logDeep("STOP", "Interrupted, exiting daemon.")
			return nil
		case update := <-updateChan:
			if update.Message != nil && update.Message.Text != "" {
				text := update.Message.Text
				re := regexp.MustCompile(`https://djinni\.co/jobs/(\d+-[a-zA-Z0-9-]+)/?`)
				match := re.FindStringSubmatch(text)
				if len(match) > 1 {
					slug := match[1]
					url := match[0]

					notify.SendTelegramMessage(fmt.Sprintf(" Processing manual job URL: %s", url))
					logger.Log.Info(" Processing manual job URL", "url", url)

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

					applied, err := processJobItem(ctx, cfg, bot, dc, engine, &pipeline.Dedup{}, j, &skippedDedupe, &skippedThreshold, &errorCount, &pdfCount, &appliedJobs)
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
