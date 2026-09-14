package main

import (
	"github.com/spf13/cobra"
)

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

func init() {
	pipelineRunCmd.Flags().Float64Var(&flagThreshold, "threshold", 4.2, "Score threshold to trigger auto-apply (0.0 to 5.0)")
	pipelineRunCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Scan and evaluate jobs, but do not send applications")
	pipelineRunCmd.Flags().IntVar(&flagLimit, "limit", 5, "Maximum number of applications to submit in this run")
	pipelineRunCmd.Flags().BoolVar(&flagDaemon, "daemon", false, "Run continuously in background, spreading up to 15 applications daily between 9 AM and 9 PM")

	pipelineInboxCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Generate replies but do not send them to recruiters")

	pipelineCmd.AddCommand(pipelineRunCmd)
	pipelineCmd.AddCommand(pipelineInboxCmd)
	rootCmd.AddCommand(pipelineCmd)
}
