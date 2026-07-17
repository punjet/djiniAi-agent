package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/eval"
	"djinni-bot-go/internal/llm"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var evaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate a job description using an LLM",
	Long: `Evaluate a job description against your CV using the A-G scoring system.

The evaluation result is printed to stdout, and (by default) a markdown report
is saved to the reports/ directory inside --context-dir.

Engines:
  gemini      Direct Gemini API (requires GEMINI_API_KEY)
  ollama      Local Ollama server (requires ollama running)
  freellmapi  Local freellmapi proxy — auto-fallback across 13 free providers
              (Groq, Cerebras, Gemini, Ollama Cloud, OpenRouter, etc.)
              Start it with: cd freellmapi && npm run dev
  openai      Direct OpenAI API (requires LLM_API_KEY, defaults to gpt-4o-mini)

Examples:
  career-ops evaluate --jd "We are looking for a Senior Go Engineer..."
  career-ops evaluate --file ./career-ops/jds/job.txt --engine ollama
  career-ops evaluate --file ./career-ops/jds/job.txt --engine freellmapi
  career-ops evaluate --file ./career-ops/jds/job.txt --no-save`,
	RunE: runEvaluate,
}

var (
	flagJD         string
	flagFile       string
	flagNoSave     bool
	flagOutputJSON bool
)

func init() {
	evaluateCmd.Flags().StringVar(&flagJD, "jd", "", "Inline job description text")
	evaluateCmd.Flags().StringVar(&flagFile, "file", "", "Path to a file containing the job description")
	evaluateCmd.Flags().BoolVar(&flagNoSave, "no-save", false, "Do not save the report to disk")
	evaluateCmd.Flags().BoolVar(&flagOutputJSON, "output-json", false, "Output results as a single JSON object to stdout")
}

func runEvaluate(cmd *cobra.Command, args []string) error {
	// -----------------------------------------------------------------------
	// 1. Resolve JD text
	// -----------------------------------------------------------------------
	jdText := strings.TrimSpace(flagJD)

	if flagFile != "" {
		data, err := os.ReadFile(flagFile)
		if err != nil {
			return fmt.Errorf("could not read JD file %q: %w", flagFile, err)
		}
		jdText = strings.TrimSpace(string(data))
	}

	// Also accept positional args (mirrors JS: node gemini-eval.mjs "text")
	if jdText == "" && len(args) > 0 {
		jdText = strings.Join(args, "\n")
	}

	if jdText == "" {
		return fmt.Errorf("no job description provided — use --jd \"text\" or --file path")
	}

	// -----------------------------------------------------------------------
	// 2. Load config from environment / .env
	// -----------------------------------------------------------------------
	_ = godotenv.Overload(filepath.Join(flagContextDir, ".env"))
	cfg, err := config.LoadConfig()
	if err != nil {
		// For career-ops, Djinni session credentials are not required.
		// Only LLM keys matter. We swallow the validation error here and
		// let the LLM factory surface the relevant error.
		cfg = config.MustLoadPartial()
	}

	// -----------------------------------------------------------------------
	// 3. Build LLM provider
	// -----------------------------------------------------------------------
	engine := llm.Engine(flagEngine)
	provider, err := llm.NewProvider(cfg, engine, "evaluation")
	if err != nil {
		return fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// For Ollama and freellmapi: probe reachability before burning time on prompt assembly
	if engine == llm.EngineOllama || engine == llm.EngineFreeLLMAPI {
		if oc, ok := provider.(*llm.OllamaClient); ok {
			serverURL := cfg.OllamaBaseURL
			if engine == llm.EngineFreeLLMAPI {
				serverURL = cfg.FreeLLMAPIBaseURL
			}
			if !flagOutputJSON {
				fmt.Fprintf(os.Stdout, "🔍  Probing %s at %s...\n", engine, serverURL)
			}
			if err := oc.Probe(context.Background()); err != nil {
				if engine == llm.EngineFreeLLMAPI {
					return fmt.Errorf("%w\n\n  Start freellmapi: cd freellmapi && npm run dev", err)
				}
				return err
			}
		}
	}

	// -----------------------------------------------------------------------
	// 4. Run evaluation
	// -----------------------------------------------------------------------
	if !flagOutputJSON {
		fmt.Fprintf(os.Stdout, "🤖  Calling %s... this may take 30-90 seconds.\n\n", provider.Name())
	}

	result, err := eval.Evaluate(context.Background(), provider, flagContextDir, jdText)
	if err != nil {
		return err
	}

	// -----------------------------------------------------------------------
	// 5. Save report
	// -----------------------------------------------------------------------
	reportFilename := ""
	if !flagNoSave {
		// FRAGILE: Output redirection was previously a goroutine-unsafe swap of os.Stdout.
		// TODO: Ensure all output writing uses explicitly passed io.Writer dependencies.
		var out io.Writer = os.Stdout
		if flagOutputJSON {
			out = os.Stderr
		}
		fname, err := eval.SaveReport(result, flagContextDir, provider.Name(), out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️   Could not save report: %v\n", err)
			os.Exit(1)
		}
		reportFilename = fname
	}

	// -----------------------------------------------------------------------
	// 6. Display result
	// -----------------------------------------------------------------------
	if flagOutputJSON {
		resObj := map[string]interface{}{
			"score":       result.Score,
			"archetype":   result.Archetype,
			"legitimacy":  result.Legitimacy,
			"company":     result.Company,
			"role":        result.Role,
			"report_path": reportFilename,
			"full_text":   result.FullText,
		}
		bytes, err := json.MarshalIndent(resObj, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON output: %w", err)
		}
		fmt.Println(string(bytes))
	} else {
		sep := strings.Repeat("═", 66)
		fmt.Println()
		fmt.Println(sep)
		fmt.Printf("  CAREER-OPS EVALUATION — powered by %s\n", provider.Name())
		fmt.Println(sep)
		fmt.Println()
		fmt.Println(result.FullText)
		fmt.Println()
		fmt.Println(strings.Repeat("─", 66))
		fmt.Printf("  Score: %.1f/5  |  Archetype: %s  |  Legitimacy: %s\n",
			result.Score, result.Archetype, result.Legitimacy)
		fmt.Println(strings.Repeat("─", 66))
		fmt.Println()
	}

	return nil
}
