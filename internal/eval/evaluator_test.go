package eval

import (
	"context"
	"os"
	"strings"
	"testing"

	"djinni-bot-go/internal/llm"
)

// ---------------------------------------------------------------------------
// validateEvaluationShape tests
// ---------------------------------------------------------------------------

// minimalValidReport creates a report containing all required blocks A-G
// plus a valid SCORE_SUMMARY section.
func minimalValidReport(score string) string {
	return `## A. Tech Stack Match
Content for block A.

## B. Role Clarity
Content for block B.

## C. Culture & Work Style
Content for block C.

## D. Compensation Research
Content for block D.

## E. Growth & Development
Content for block E.

## F. Red Flags
Content for block F.

## G. Legitimacy
Content for block G.

---SCORE_SUMMARY---
COMPANY: Acme Corp
ROLE: Senior Go Engineer
SCORE: ` + score + `
ARCHETYPE: Tech Lead
LEGITIMACY: High Confidence
---END_SUMMARY---`
}

func TestValidateEvaluationShape_Valid(t *testing.T) {
	if err := validateEvaluationShape(minimalValidReport("3.8")); err != nil {
		t.Errorf("expected valid report to pass, got: %v", err)
	}
}

func TestValidateEvaluationShape_MissingBlockC(t *testing.T) {
	report := strings.Replace(minimalValidReport("3.8"), "## C. Culture & Work Style\nContent for block C.\n\n", "", 1)
	err := validateEvaluationShape(report)
	if err == nil {
		t.Fatal("expected error for missing Block C, got nil")
	}
	if !strings.Contains(err.Error(), "missing Block C") {
		t.Errorf("expected 'missing Block C' in error, got: %v", err)
	}
}

func TestValidateEvaluationShape_MissingBlock(t *testing.T) {
	// Remove Block D
	report := strings.Replace(minimalValidReport("3.8"), "## D. Compensation Research\nContent for block D.\n\n", "", 1)
	err := validateEvaluationShape(report)
	if err == nil {
		t.Fatal("expected error for missing Block D, got nil")
	}
	if !strings.Contains(err.Error(), "missing Block D") {
		t.Errorf("expected 'missing Block D' in error, got: %v", err)
	}
}

func TestValidateEvaluationShape_MissingSummary(t *testing.T) {
	report := `## A. Match
Content A.
## B. Clarity
Content B.
## C. Culture
Content C.
## D. Comp
Content D.
## E. Growth
Content E.
## F. Red Flags
Content F.
## G. Legitimacy
Content G.`
	err := validateEvaluationShape(report)
	if err == nil {
		t.Fatal("expected error for missing SCORE_SUMMARY, got nil")
	}
	if !strings.Contains(err.Error(), "missing SCORE_SUMMARY block") {
		t.Errorf("expected 'missing SCORE_SUMMARY block', got: %v", err)
	}
}

func TestValidateEvaluationShape_ScoreOutOfRange(t *testing.T) {
	err := validateEvaluationShape(minimalValidReport("6.0"))
	if err == nil {
		t.Fatal("expected error for score > 5, got nil")
	}
	if !strings.Contains(err.Error(), "score must be a number between 0 and 5") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateEvaluationShape_ScoreNegative(t *testing.T) {
	err := validateEvaluationShape(minimalValidReport("-1"))
	if err == nil {
		t.Fatal("expected error for negative score, got nil")
	}
}

func TestValidateEvaluationShape_BoldMarkers(t *testing.T) {
	// LLM sometimes wraps block headings and values in **bold** markers.
	report := `### **A) Tech Stack Match**
Content for block A.

### **B) Role Clarity**
Content for block B.

### **C) Culture & Work Style**
Content for block C.

### **D) Compensation Research**
Content for block D.

### **E) Growth & Development**
Content for block E.

### **F) Red Flags**
Content for block F.

### **G) Legitimacy**
Content for block G.

---SCORE_SUMMARY---
COMPANY: **Acme Corp**
ROLE: **Senior Go Engineer**
SCORE: **3.8**
ARCHETYPE: **Tech Lead**
LEGITIMACY: **High Confidence**
---END_SUMMARY---`
	if err := validateEvaluationShape(report); err != nil {
		t.Errorf("expected bold-marked report to pass, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// parseScoreSummary tests
// ---------------------------------------------------------------------------

func TestParseScoreSummary_FullMatch(t *testing.T) {
	result := parseScoreSummary(minimalValidReport("4.2"))
	if result.Company != "Acme Corp" {
		t.Errorf("Company: got %q, want %q", result.Company, "Acme Corp")
	}
	if result.Role != "Senior Go Engineer" {
		t.Errorf("Role: got %q, want %q", result.Role, "Senior Go Engineer")
	}
	if result.Score != 4.2 {
		t.Errorf("Score: got %v, want 4.2", result.Score)
	}
	if result.Archetype != "Tech Lead" {
		t.Errorf("Archetype: got %q, want %q", result.Archetype, "Tech Lead")
	}
	if result.Legitimacy != "High Confidence" {
		t.Errorf("Legitimacy: got %q, want %q", result.Legitimacy, "High Confidence")
	}
	if result.Summary != "Content for block A." {
		t.Errorf("Summary: got %q, want %q", result.Summary, "Content for block A.")
	}
}

func TestParseScoreSummary_NoSummary(t *testing.T) {
	result := parseScoreSummary("Some text without a summary block.")
	if result.Company != "unknown" || result.Score != 0 {
		t.Errorf("expected default values for missing summary, got: %+v", result)
	}
}

func TestParseScoreSummary_BoldMarkers(t *testing.T) {
	report := `---SCORE_SUMMARY---
COMPANY: **Acme Corp**
ROLE: **Senior Go Engineer**
SCORE: **4.2**
ARCHETYPE: **Tech Lead**
LEGITIMACY: **High Confidence**
---END_SUMMARY---`
	result := parseScoreSummary(report)
	if result.Company != "Acme Corp" {
		t.Errorf("Company: got %q, want %q", result.Company, "Acme Corp")
	}
	if result.Role != "Senior Go Engineer" {
		t.Errorf("Role: got %q, want %q", result.Role, "Senior Go Engineer")
	}
	if result.Score != 4.2 {
		t.Errorf("Score: got %v, want 4.2", result.Score)
	}
	if result.Archetype != "Tech Lead" {
		t.Errorf("Archetype: got %q, want %q", result.Archetype, "Tech Lead")
	}
	if result.Legitimacy != "High Confidence" {
		t.Errorf("Legitimacy: got %q, want %q", result.Legitimacy, "High Confidence")
	}
}

func TestParseScoreSummary_SummaryFormattingEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "single line without newline",
			input: `### A) This is a single line summary without newline
## B. Role Clarity
---SCORE_SUMMARY---
COMPANY: Acme Corp
ROLE: Senior Go Engineer
SCORE: 4.2
ARCHETYPE: Tech Lead
LEGITIMACY: High Confidence
---END_SUMMARY---`,
			expected: "This is a single line summary without newline",
		},
		{
			name: "single line with newline at end",
			input: `### A) This is a single line summary with newline at end
## B. Role Clarity
---SCORE_SUMMARY---
COMPANY: Acme Corp
ROLE: Senior Go Engineer
SCORE: 4.2
ARCHETYPE: Tech Lead
LEGITIMACY: High Confidence
---END_SUMMARY---`,
			expected: "This is a single line summary with newline at end",
		},
		{
			name: "long first line kept",
			input: `### A) The candidate is a strong match for this role, with extensive experience in Go development.
They also have strong experience with AWS and Kubernetes.
## B. Role Clarity
---SCORE_SUMMARY---
COMPANY: Acme Corp
ROLE: Senior Go Engineer
SCORE: 4.2
ARCHETYPE: Tech Lead
LEGITIMACY: High Confidence
---END_SUMMARY---`,
			expected: "The candidate is a strong match for this role, with extensive experience in Go development.\nThey also have strong experience with AWS and Kubernetes.",
		},
		{
			name: "short header line skipped",
			input: `### A) Tech Stack Match
The candidate is a strong match.
## B. Role Clarity
---SCORE_SUMMARY---
COMPANY: Acme Corp
ROLE: Senior Go Engineer
SCORE: 4.2
ARCHETYPE: Tech Lead
LEGITIMACY: High Confidence
---END_SUMMARY---`,
			expected: "The candidate is a strong match.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := parseScoreSummary(tc.input)
			if res.Summary != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, res.Summary)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// slugifyCompany tests
// ---------------------------------------------------------------------------

func TestSlugifyCompany(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Acme Corp.", "acme-corp"},
		{"Google LLC", "google-llc"},
		{"", "unknown"},
		{"  ---  ", "unknown"},
		{"ABC & XYZ", "abc-xyz"},
	}
	for _, c := range cases {
		got := slugifyCompany(c.input)
		if got != c.want {
			t.Errorf("slugifyCompany(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// nextReportNumber tests
// ---------------------------------------------------------------------------

func TestNextReportNumber_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	num, err := nextReportNumber(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != "001" {
		t.Errorf("want 001, got %s", num)
	}
}

func TestNextReportNumber_WithFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001-acme-2026-01-01.md", "005-google-2026-02-01.md"} {
		path := dir + "/" + name
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	num, err := nextReportNumber(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if num != "006" {
		t.Errorf("want 006, got %s", num)
	}
}

// ---------------------------------------------------------------------------
// Evaluate tests
// ---------------------------------------------------------------------------

func TestEvaluate_WithMock(t *testing.T) {
	ctx := context.Background()
	mock := &llm.MockProvider{
		GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
			return minimalValidReport("4.2"), nil
		},
		ProviderName: "TestMock",
	}

	tmpContextDir := t.TempDir()
	if err := os.MkdirAll(tmpContextDir+"/modes", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tmpContextDir+"/config", 0755); err != nil {
		t.Fatal(err)
	}

	result, err := Evaluate(ctx, mock, tmpContextDir, "Some job text")
	if err != nil {
		t.Fatalf("unexpected error from Evaluate: %v", err)
	}

	if result.Score != 4.2 {
		t.Errorf("expected score 4.2, got %v", result.Score)
	}
	if result.Company != "Acme Corp" {
		t.Errorf("expected company Acme Corp, got %v", result.Company)
	}
}
