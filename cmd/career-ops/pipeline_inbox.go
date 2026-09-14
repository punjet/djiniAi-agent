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
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/logger"
	"djinni-bot-go/internal/notify"
	"djinni-bot-go/internal/pipeline"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func runPipelineInbox(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

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
		return fmt.Errorf("failed to load Djinni config (session credentials required to read/reply inbox): %w", err)
	}

	dc := client.NewDjinniClient(cfg)
	engine := llm.Engine(flagEngine)

	bot := notify.NewTelegramBot()
	bot.Start()
	defer bot.Stop()
	setupBotCommands(bot, dc, ctx)

	if !api.CheckToken(dc) {
		notify.SendTelegramMessage(" Djinni sessionid cookie expired or invalid! Send `/set_session <your_sessionid>` to update it.")
		return fmt.Errorf("invalid token, cannot process inbox")
	}

	logger.Log.Info("  Scanning inbox for unread dialogue messages...")
	logs, err := pipeline.ProcessInbox(ctx, bot, sigChan, cfg, engine, flagContextDir, dc, flagDryRun)
	if err != nil {
		return err
	}

	repliedCount := 0
	skippedCount := 0
	errorCount := 0
	var summary strings.Builder
	summary.WriteString(" *Recruiter Inbox Processed*\n")
	summary.WriteString(fmt.Sprintf(" Date: %s\n\n", time.Now().Format("2006-01-02 15:04")))

	for _, logLine := range logs {
		logger.Log.Info(fmt.Sprint(logLine))
		if strings.Contains(logLine, "Reply:") {
			repliedCount++
			summary.WriteString(fmt.Sprintf(" %s\n", logLine))
		} else if strings.Contains(logLine, "Skipped") {
			skippedCount++
		} else {
			errorCount++
			summary.WriteString(fmt.Sprintf(" %s\n", logLine))
		}
	}

	summary.WriteString(fmt.Sprintf("\n *Summary:* Replied: %d | Skipped: %d | Errors: %d", repliedCount, skippedCount, errorCount))

	// Only send a TG message if we actually replied or had errors, avoiding empty check spam
	if repliedCount > 0 || errorCount > 0 {
		_ = notify.SendTelegramMessage(summary.String())
	}

	if bot != nil {
		bot.SetLastSummary(summary.String())
	}

	return nil
}
