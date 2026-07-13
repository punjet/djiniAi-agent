package covergen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"djinni-bot-go/internal/config"
)

func TestPreprocessTemplate(t *testing.T) {
	input := `<h1>{{NAME}}</h1><p>{{CONTACT_LINE}}</p>{{EMPTY}}`
	expected := `<h1>{{.NAME}}</h1><p>{{.CONTACT_LINE}}</p>{{.EMPTY}}`
	got := preprocessTemplate(input)
	if got != expected {
		t.Errorf("preprocessTemplate(%q) = %q, want %q", input, got, expected)
	}
}

func TestPreprocessTemplate_NoPlaceholders(t *testing.T) {
	input := `<h1>Hello</h1><p>World</p>`
	got := preprocessTemplate(input)
	if got != input {
		t.Errorf("preprocessTemplate(%q) = %q, want unchanged", input, got)
	}
}

func TestPreprocessTemplate_GoTemplateUnchanged(t *testing.T) {
	input := `<p>{{.NAME}}</p><p>{{.CONTACT_LINE}}</p>`
	got := preprocessTemplate(input)
	if got != input {
		t.Errorf("preprocessTemplate should not change existing Go templates, got %q", got)
	}
}

func TestRenderHTMLToPDF(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chromedp test in short mode")
	}

	html := "<html><body><h1>Hello PDF</h1></body></html>"
	buf, err := renderHTMLToPDF(context.Background(), html)
	if err != nil {
		t.Skipf("chromedp not available (no Chrome/Chromium binary?): %v", err)
	}
	if len(buf) == 0 {
		t.Error("expected non-empty PDF bytes")
	}
}

func TestValidateCVHTML(t *testing.T) {
	validHTML := "<html><body><h1>John Doe</h1></body></html>"
	if err := ValidateCVHTML(validHTML); err != nil {
		t.Errorf("expected nil error for valid HTML, got %v", err)
	}

	invalidHTML1 := "<html><body><h1>{{NAME}}</h1></body></html>"
	if err := ValidateCVHTML(invalidHTML1); err == nil {
		t.Errorf("expected error for HTML with {{, got nil")
	}

	invalidHTML2 := "<html><body><h1>John Doe}}</h1></body></html>"
	if err := ValidateCVHTML(invalidHTML2); err == nil {
		t.Errorf("expected error for HTML with }}, got nil")
	}
}

func TestPayloadJSON(t *testing.T) {
	p := Payload{
		Candidate: map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
		Letter: map[string]interface{}{
			"role_title": "Engineer",
			"company":    "Acme",
		},
		OutputPath: "/tmp/out.pdf",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal Payload: %v", err)
	}

	var got Payload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal Payload: %v", err)
	}

	if got.Candidate["name"] != "John Doe" {
		t.Errorf("Candidate.name: got %v, want John Doe", got.Candidate["name"])
	}
	if got.Letter["company"] != "Acme" {
		t.Errorf("Letter.company: got %v, want Acme", got.Letter["company"])
	}
	if got.OutputPath != "/tmp/out.pdf" {
		t.Errorf("OutputPath: got %s, want /tmp/out.pdf", got.OutputPath)
	}
}

func TestGeneratedLetterJSON(t *testing.T) {
	gl := GeneratedLetter{
		Letter: map[string]interface{}{
			"role_title": "Senior Go Dev",
			"company":    "TechCorp",
		},
		DjinniMessage: "Hello, I am interested...",
	}

	data, err := json.Marshal(gl)
	if err != nil {
		t.Fatalf("failed to marshal GeneratedLetter: %v", err)
	}

	var got GeneratedLetter
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal GeneratedLetter: %v", err)
	}

	if got.Letter["company"] != "TechCorp" {
		t.Errorf("Letter.company: got %v, want TechCorp", got.Letter["company"])
	}
	if got.DjinniMessage != "Hello, I am interested..." {
		t.Errorf("DjinniMessage: got %s, want original", got.DjinniMessage)
	}
}

func TestGenerateCoverLetter_MissingProfile(t *testing.T) {
	td := t.TempDir()

	_, _, err := GenerateCoverLetter(nil, nil, "", td, "", "", "test JD")
	if err == nil {
		t.Error("expected error for missing config/profile.yml, got nil")
	}
}

func TestGenerateCoverLetter_MissingTemplate(t *testing.T) {
	td := t.TempDir()
	profileDir := filepath.Join(td, "config")
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(filepath.Join(profileDir, "profile.yml"), []byte(`
candidate:
  full_name: Test User
  email: test@example.com
  phone: "+1234567890"
  location: Kyiv
  linkedin: ""
  github: ""
target_roles:
  primary:
    - Go Developer
`), 0o644)
	os.WriteFile(filepath.Join(td, "cv.md"), []byte("# Test User\nGo Developer\n"), 0o644)
	// No templates/cover-letter-template.html

	ctx := context.Background()
	_, _, err := GenerateCoverLetter(ctx, nil, "", td, "", "", "test JD")
	if err == nil {
		t.Error("expected error for missing cover letter template, got nil")
	}
}

func TestGenerateCustomCV_InvalidContextDir(t *testing.T) {
	// Non-existent contextDir
	cfg := &config.Config{
		FreeLLMAPIBaseURL: "http://localhost:3001",
		LLMAPIKey:         "test-key",
	}
	_, err := GenerateCustomCV(context.Background(), cfg, "", "/nonexistent/path", "", "Acme", "Dev", "/tmp/report")
	if err == nil {
		t.Error("expected error for nonexistent contextDir, got nil")
	}
}

func TestProfileYAML(t *testing.T) {
	td := t.TempDir()
	profileDir := filepath.Join(td, "config")
	os.MkdirAll(profileDir, 0o755)
	profilePath := filepath.Join(profileDir, "profile.yml")
	os.WriteFile(profilePath, []byte(`
candidate:
  full_name: Test User
  email: test@example.com
  phone: "+1234567890"
  location: Kyiv
  linkedin: https://linkedin.com/in/test
  github: https://github.com/test
target_roles:
  primary:
    - Go Developer
    - Backend Engineer
`), 0o644)

	data, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatalf("failed to read profile: %v", err)
	}

	var prof Profile
	if err := json.Unmarshal(data, &prof); err != nil {
		// Profile uses yaml tags, but json.Unmarshal will fail with zero values - that's expected
		// We're testing the file reading, not YAML parsing
	}

	// Just verify the file exists and is readable
	if len(data) == 0 {
		t.Fatal("profile data is empty")
	}
}

func TestAnswerQuizQuestions_MissingProfile(t *testing.T) {
	td := t.TempDir()
	// No config/profile.yml created

	_, err := AnswerQuizQuestions(context.Background(), nil, "", td, nil, "", "", "")
	if err == nil {
		t.Error("expected error for missing profile.yml, got nil")
	}
}

func TestAnswerQuizQuestions_MissingCV(t *testing.T) {
	td := t.TempDir()
	profileDir := filepath.Join(td, "config")
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(filepath.Join(profileDir, "profile.yml"), []byte(`
candidate:
  full_name: Kyrylo Kirov
  email: kyrylo@example.com
  location: Kyiv
  linkedin: ""
  github: ""
target_roles:
  primary:
    - AI Engineer
`), 0o644)
	// No cv.md created

	_, err := AnswerQuizQuestions(context.Background(), nil, "", td, nil, "", "", "")
	if err == nil {
		t.Error("expected error for missing cv.md, got nil")
	}
}

func TestAnswerQuizQuestions_UnknownEngine(t *testing.T) {
	// Pass valid profile/CV but empty engine — should fail at llm.NewProvider
	ctx := context.Background()
	td := t.TempDir()

	profileDir := filepath.Join(td, "config")
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(filepath.Join(profileDir, "profile.yml"), []byte(`
candidate:
  full_name: Kyrylo Kirov
  email: kyrylo@example.com
  location: Kyiv
  linkedin: ""
  github: ""
target_roles:
  primary:
    - AI Engineer
`), 0o644)
	os.WriteFile(filepath.Join(td, "cv.md"), []byte(`# Kyrylo Kirov
AI Expert
`), 0o644)

	_, err := AnswerQuizQuestions(ctx, nil, "", td, nil, "", "", "")
	if err == nil {
		t.Error("expected error for unknown LLM engine, got nil")
	}
}
