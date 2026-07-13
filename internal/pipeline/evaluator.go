package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/eval"
	"djinni-bot-go/internal/llm"
)

type EvalResult struct {
	Score      float64
	Archetype  string
	Legitimacy string
	Company    string
	Role       string
	ReportPath string
	FullText   string
}

// EvaluateJob performs an in-process evaluation of a job description,
// saves the markdown report and TSV tracker addition, and returns the result.
func EvaluateJob(ctx context.Context, jdText string, cfg *config.Config, engine llm.Engine, contextDir string) (*EvalResult, error) {
	provider, err := llm.NewProvider(cfg, engine)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	result, err := eval.Evaluate(ctx, provider, contextDir, jdText)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	// Save report
	reportFilename, err := eval.SaveReport(result, contextDir, provider.Name(), os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to save report: %w", err)
	}

	return &EvalResult{
		Score:      result.Score,
		Archetype:  result.Archetype,
		Legitimacy: result.Legitimacy,
		Company:    result.Company,
		Role:       result.Role,
		ReportPath: filepath.Join("reports", reportFilename),
		FullText:   result.FullText,
	}, nil
}
