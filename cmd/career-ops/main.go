package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the top-level cobra command for career-ops.
var rootCmd = &cobra.Command{
	Use:   "career-ops",
	Short: "Career-Ops — AI-powered job evaluation and cover letter generation",
	Long: `Career-Ops is a CLI tool that evaluates job descriptions against your CV
using Gemini or Ollama, generates cover letters, and saves structured reports.

Set GEMINI_API_KEY or configure Ollama before running.`,
}

// Global flags (available to all subcommands)
var (
	flagEngine     string
	flagContextDir string
)

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&flagEngine, "engine", "e", "gemini",
		`LLM engine to use: "gemini", "ollama", "freellmapi", "openai"`,
	)
	rootCmd.PersistentFlags().StringVarP(
		&flagContextDir, "context-dir", "d", "./career-ops",
		"Path to the career-ops directory containing cv.md, modes/, config/",
	)

	rootCmd.AddCommand(evaluateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
