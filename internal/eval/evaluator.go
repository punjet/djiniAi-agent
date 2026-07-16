package eval

import (
	"context"
	"fmt"
	// "log" // TODO: Re-enable langdetect and log once go get issue is resolved.
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/config" // Added missing import
	// "github.com/rylans/langdetect"
	// _ "github.com/rylans/langdetect/profiles"
)

// EvalResult holds the parsed output of a job evaluation.
type EvalResult struct {
	Company    string
	Role       string
	Score      float64
	Archetype  string
	Legitimacy string
	FullText   string // the complete LLM response
}

// contextFiles bundles all prompt-context files loaded from disk.
type contextFiles struct {
	shared     string
	oferta     string
	cv         string
	profile    string
	profileYml string
}

// Evaluate runs the full evaluation pipeline:
//  1. Loads context files from contextDir
//  2. Builds the system prompt (mirrors gemini-eval.mjs / ollama-eval.mjs)
//  3. Calls the LLM provider
//  4. Validates the A-G block structure and SCORE_SUMMARY
//  5. Parses and returns EvalResult
func Evaluate(ctx context.Context, cfg *config.Config, contextDir, jdText string) (*EvalResult, error) {
	provider, err := llm.NewProvider(cfg, llm.EngineOpenAI, "evaluation")

	cf, err := loadContextFiles(contextDir)
	if err != nil {
		return nil, fmt.Errorf("loading context files: %w", err)
	}

	systemPrompt := buildSystemPrompt(cf)
	userMessage := "JOB DESCRIPTION TO EVALUATE:\n\n" + jdText

	text, err := provider.GenerateText(ctx, systemPrompt, userMessage)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if err := validateEvaluationShape(text); err != nil {
		return nil, fmt.Errorf("LLM output failed validation: %w", err)
	}

	result := parseScoreSummary(text)
	result.FullText = text
	return result, nil
}

// ---------------------------------------------------------------------------
// Context loading
// ---------------------------------------------------------------------------

// readFile reads a file from disk. If the file does not exist, it returns a
// placeholder string (mirrors the JS readFile helper).
func readFile(path, label string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️   %s not found at: %s\n", label, path)
		return fmt.Sprintf("[%s not found — skipping]", label)
	}
	return strings.TrimSpace(string(data))
}

func loadContextFiles(contextDir string) (contextFiles, error) {
	return contextFiles{
		shared:     readFile(filepath.Join(contextDir, "modes", "_shared.md"), "modes/_shared.md"),
		oferta:     readFile(filepath.Join(contextDir, "modes", "oferta.md"), "modes/oferta.md"),
		cv:         readFile(filepath.Join(contextDir, "cv.md"), "cv.md"),
		profile:    readFile(filepath.Join(contextDir, "modes", "_profile.md"), "modes/_profile.md"),
		profileYml: readFile(filepath.Join(contextDir, "config", "profile.yml"), "config/profile.yml"),
	}, nil
}

// ---------------------------------------------------------------------------
// Prompt building
// ---------------------------------------------------------------------------

func buildSystemPrompt(cf contextFiles) string {
	// TODO: Re-enable langdetect logic and calls here once the 'go get' issue is resolved.
	// For now, this function directly builds the prompt without language validation.

	sep := strings.Repeat("═", 55)
	return fmt.Sprintf(`You are career-ops, an AI-powered job search assistant.
You evaluate job offers against the user\'s CV using a structured A-G scoring system.

Your evaluation methodology is defined below. Follow it exactly.

%s
SYSTEM CONTEXT (_shared.md)
%s
%s

%s
EVALUATION MODE (oferta.md)
%s
%s

%s
CANDIDATE RESUME (cv.md)
%s
%s

%s
CANDIDATE PROFILE & TARGETS (config/profile.yml)
%s
%s

%s
USER ARCHETYPES & NARRATIVE (_profile.md)
%s
%s

%s
IMPORTANT OPERATING RULES FOR THIS CLI SESSION
%s
1. You do NOT have access to WebSearch, Playwright, or file writing tools.
   - For Block D (Comp research): provide salary estimates based on your training data, clearly noted as estimates.
   - For Block G (Legitimacy): analyze the JD text only; skip URL/page freshness checks.
   - Post-evaluation file saving is handled by the script, not by you.
2. CRITICAL FILTERING RULE: If the job description is for a "classic" software engineer (e.g., standard Python backend, standard React frontend, standard Java enterprise) with NO focus on AI Agents, LLM Integrations, Process Automation (n8n, Make), or AI features, you MUST give it a SCORE below 3.0. The candidate is an AI Automation Expert / AI Integrator, not a generic developer.
3. Generate Blocks A through G in [[JD_LANG]] language (Ukrainian or English, NEVER Russian). Ensure the summary is detailed and at least 200 words long. The SCORE_SUMMARY block must keep its machine-parseable English format (COMPANY:, ROLE:, SCORE:, ARCHETYPE:, LEGITIMACY:). All analysis blocks A-G MUST be in [[JD_LANG]] language.
4. At the very end, output a machine-readable summary block in this exact format:

---SCORE_SUMMARY---
COMPANY: <company name or "Unknown">
ROLE: <role title>
SCORE: <global score as decimal, e.g. 3.8>
ARCHETYPE: <detected archetype>
LEGITIMACY: <High Confidence | Proceed with Caution | Suspicious>
---END_SUMMARY---
`,
		sep, sep, cf.shared,
		sep, sep, cf.oferta,
		sep, sep, cf.cv,
		sep, sep, cf.profileYml,
		sep, sep, cf.profile,
		sep, sep,
	)
}

// ---------------------------------------------------------------------------
// Validation (port of validateEvaluationShape from gemini-eval.mjs)
// ---------------------------------------------------------------------------

var blockPatterns = []struct {
	label   string
	pattern *regexp.Regexp
}{
	// Matches English: "### A)", "### Block A", "**A)**", "A)" — and Russian: "**Блок A:**", "## Блок A"
	{"A", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:A\b|Block\s+A\b|Блок\s+A\b)\s*(?:\*\*)?`)},
	{"B", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:B\b|Block\s+B\b|Блок\s+B\b)\s*(?:\*\*)?`)},
	{"C", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:C\b|Block\s+C\b|Блок\s+C\b)\s*(?:\*\*)?`)},
	{"D", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:D\b|Block\s+D\b|Блок\s+D\b)\s*(?:\*\*)?`)},
	{"E", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:E\b|Block\s+E\b|Блок\s+E\b)\s*(?:\*\*)?`)},
	{"F", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:F\b|Block\s+F\b|Блок\s+F\b)\s*(?:\*\*)?`)},
	{"G", regexp.MustCompile(`(?im)(?:^|\n)(?:#{1,3}\s*)?(?:\*\*)?\s*(?:G\b|Block\s+G\b|Блок\s+G\b)\s*(?:\*\*)?`)},
}

// stripBoldMarkers removes ** markers from text so shape validation and
// score/field parsing are not confused by bold markdown around headings or
// values (e.g., "### **A) ...**" or "SCORE: **3.8**").
func stripBoldMarkers(text string) string {
	return strings.ReplaceAll(text, "**", "")
}

var summaryBlockRe = regexp.MustCompile(`(?s)---SCORE_SUMMARY---\s*(.*?)---END_SUMMARY---`)
var scoreRe = regexp.MustCompile(`(?im)^\s*SCORE:\s*([0-9]+(?:\.[0-9]+)?)`)

func validateEvaluationShape(text string) error {
	// Strip bold markers so that "### **A) ...**" and "SCORE: **3.8**" match.
	clean := stripBoldMarkers(text)

	var issues []string

	for _, b := range blockPatterns {
		if !b.pattern.MatchString(clean) {
			issues = append(issues, fmt.Sprintf("missing Block %s", b.label))
		}
	}

	m := summaryBlockRe.FindStringSubmatch(clean)
	if m == nil {
		issues = append(issues, "missing SCORE_SUMMARY block")
	} else {
		summaryBlock := m[1]
		for _, key := range []string{"COMPANY", "ROLE", "ARCHETYPE", "LEGITIMACY"} {
			re := regexp.MustCompile(`(?im)^\s*` + key + `:\s*(.+)$`)
			match := re.FindStringSubmatch(summaryBlock)
			val := ""
			if len(match) > 1 {
				val = strings.TrimSpace(match[1])
			}
			if val == "" || (key != "COMPANY" && strings.EqualFold(val, "unknown")) {
				issues = append(issues, fmt.Sprintf("SCORE_SUMMARY %s is required", key))
			}
		}

		scoreMatch := scoreRe.FindStringSubmatch(summaryBlock)
		if len(scoreMatch) < 2 {
			issues = append(issues, "SCORE_SUMMARY score is missing")
		} else {
			scoreVal, err := strconv.ParseFloat(scoreMatch[1], 64)
			if err != nil || scoreVal < 0 || scoreVal > 5 {
				issues = append(issues, "SCORE_SUMMARY score must be a number between 0 and 5")
			}
		}
	}

	if len(issues) > 0 {
		return fmt.Errorf("LLM returned an invalid career-ops report: %s", strings.Join(issues, "; "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Score summary parsing
// ---------------------------------------------------------------------------

func parseScoreSummary(text string) *EvalResult {
	result := &EvalResult{
		Company:    "unknown",
		Role:       "unknown",
		Score:      0,
		Archetype:  "unknown",
		Legitimacy: "unknown",
	}

	// Strip bold markers so "SCORE: **3.8**" and "COMPANY: **Acme**" parse.
	clean := stripBoldMarkers(text)

	m := summaryBlockRe.FindStringSubmatch(clean)
	if m == nil {
		return result
	}
	block := m[1]

	extract := func(key string) string {
		re := regexp.MustCompile(`(?im)^\s*` + key + `:\s*(.+)$`)
		match := re.FindStringSubmatch(block)
		if len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
		return "unknown"
	}

	result.Company = extract("COMPANY")
	result.Role = extract("ROLE")
	result.Archetype = extract("ARCHETYPE")
	result.Legitimacy = extract("LEGITIMACY")

	scoreStr := extract("SCORE")
	if v, err := strconv.ParseFloat(scoreStr, 64); err == nil {
		result.Score = v
	}

	return result
}
