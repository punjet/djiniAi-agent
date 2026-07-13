package eval

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SaveReport writes the evaluation report to reports/ and a TSV tracker entry
// to batch/tracker-additions/. Mirrors the save logic from gemini-eval.mjs.
// Returns the saved report filename (e.g. "042-company-date.md") and error.
// FRAGILE: Previous implementation manipulated os.Stdout directly. Now requires an io.Writer.
// TODO: Consider moving reporting and formatting logic to a structured logger.
func SaveReport(result *EvalResult, contextDir, toolLabel string, out io.Writer) (string, error) {
	reportsDir := filepath.Join(contextDir, "reports")
	trackerDir := filepath.Join(contextDir, "batch", "tracker-additions")

	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create reports dir: %w", err)
	}
	if err := os.MkdirAll(trackerDir, 0o755); err != nil {
		return "", fmt.Errorf("could not create tracker-additions dir: %w", err)
	}

	num, err := nextReportNumber(reportsDir)
	if err != nil {
		return "", fmt.Errorf("could not determine report number: %w", err)
	}

	today := time.Now().Format("2006-01-02")
	companySlug := slugifyCompany(result.Company)
	filename := fmt.Sprintf("%s-%s-%s.md", num, companySlug, today)
	reportPath := filepath.Join(reportsDir, filename)
	trackerPath := filepath.Join(trackerDir, fmt.Sprintf("%s-%s.tsv", num, companySlug))

	// Build report markdown
	scoreStr := fmt.Sprintf("%.1f", result.Score)
	cleanBody := summaryBlockRe.ReplaceAllString(result.FullText, "")
	cleanBody = strings.TrimSpace(cleanBody)

	reportContent := fmt.Sprintf(
		"# Evaluation: %s — %s\n\n"+
			"**Date:** %s\n"+
			"**Archetype:** %s\n"+
			"**Score:** %s/5\n"+
			"**Legitimacy:** %s\n"+
			"**PDF:** pending\n"+
			"**Tool:** %s\n\n"+
			"---\n\n"+
			"%s\n",
		result.Company, result.Role,
		today,
		result.Archetype,
		scoreStr,
		result.Legitimacy,
		toolLabel,
		cleanBody,
	)

	if err := os.WriteFile(reportPath, []byte(reportContent), 0o644); err != nil {
		return "", fmt.Errorf("could not write report: %w", err)
	}
	if out != nil {
		fmt.Fprintf(out, "\n✅  Report saved: reports/%s\n", filename)
	}

	// Build TSV tracker entry
	numInt, _ := strconv.Atoi(num) // for the numeric column
	fields := []string{
		strconv.Itoa(numInt),
		today,
		tsvSafe(result.Company),
		tsvSafe(result.Role),
		"Evaluated",
		normalizedScore(scoreStr),
		"❌",
		fmt.Sprintf("[%s](reports/%s)", num, filename),
		toolLabel + " evaluation",
	}
	tsv := strings.Join(fields, "\t") + "\n"

	if err := os.WriteFile(trackerPath, []byte(tsv), 0o644); err != nil {
		return "", fmt.Errorf("could not write tracker entry: %w", err)
	}
	if out != nil {
		fmt.Fprintf(out, "📊  Tracker addition saved: batch/tracker-additions/%s-%s.tsv\n", num, companySlug)
	}

	return filename, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var reportFileRe = regexp.MustCompile(`^(\d+)-`)

// nextReportNumber scans the reports/ directory and returns the next zero-padded
// report number as a string, e.g. "042" or "1001".
func nextReportNumber(reportsDir string) (string, error) {
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "001", nil
		}
		return "", err
	}

	max := 0
	for _, e := range entries {
		m := reportFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err == nil && n > max {
			max = n
		}
	}

	next := max + 1
	numStr := strconv.Itoa(next)
	// Pad to at least 3 digits
	for len(numStr) < 3 {
		numStr = "0" + numStr
	}
	return numStr, nil
}

// slugifyCompany converts a company name to a URL-safe slug.
func slugifyCompany(name string) string {
	slug := strings.ToLower(name)
	nonAlnum := regexp.MustCompile(`[^a-z0-9]+`)
	slug = nonAlnum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "unknown"
	}
	return slug
}

// tsvSafe replaces tab/newline characters with a space, safe for TSV output.
func tsvSafe(value string) string {
	replacer := strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
	return strings.TrimSpace(replacer.Replace(value))
}

// normalizedScore ensures the score string is in "X.X/5" format.
func normalizedScore(value string) string {
	clean := tsvSafe(value)
	if clean == "" || clean == "?" {
		return "N/A"
	}
	if strings.HasSuffix(strings.ToLower(clean), "/5") {
		return clean
	}
	return clean + "/5"
}
