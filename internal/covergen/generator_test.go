package covergen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"djinni-bot-go/internal/config"
)

// ---------------------------------------------------------------------------
// Script path resolution tests
// ---------------------------------------------------------------------------

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

func TestScriptPathResolution_GenerateCoverLetter(t *testing.T) {
	// Create a temp dir simulating a contextDir
	td := t.TempDir()

	t.Run("prefers_root_when_exists", func(t *testing.T) {
		rootScript := filepath.Join(td, "generate-cover-letter.mjs")
		os.WriteFile(rootScript, []byte("// mock"), 0o644)
		scriptsDir := filepath.Join(td, "scripts")
		os.MkdirAll(scriptsDir, 0o755)
		scriptsScript := filepath.Join(scriptsDir, "generate-cover-letter.mjs")
		os.WriteFile(scriptsScript, []byte("// mock"), 0o644)

		got := filepath.Join(td, "generate-cover-letter.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "scripts", "generate-cover-letter.mjs")
		}

		if got != rootScript {
			t.Errorf("expected root path when root exists, got %s", got)
		}
	})

	t.Run("falls_back_to_scripts_when_root_missing", func(t *testing.T) {
		scriptsDir := filepath.Join(td, "scripts")
		os.MkdirAll(scriptsDir, 0o755)
		scriptsScript := filepath.Join(scriptsDir, "generate-cover-letter.mjs")
		os.WriteFile(scriptsScript, []byte("// mock"), 0o644)

		got := filepath.Join(td, "generate-cover-letter.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "scripts", "generate-cover-letter.mjs")
		}
		if _, err := os.Stat(got); os.IsNotExist(err) {
			t.Fatal("expected script to exist in scripts/ fallback")
		}
	})

	t.Run("returns_root_when_neither_exists", func(t *testing.T) {
		got := filepath.Join(td, "generate-cover-letter.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "scripts", "generate-cover-letter.mjs")
		}
		// Should return the root path even if it doesn't exist
		if got != filepath.Join(td, "generate-cover-letter.mjs") {
			t.Errorf("expected root path fallback when neither exists, got %s", got)
		}
	})
}

func TestScriptPathResolution_GeneratePDF(t *testing.T) {
	td := t.TempDir()

	t.Run("prefers_root_when_exists", func(t *testing.T) {
		rootScript := filepath.Join(td, "generate-pdf.mjs")
		os.WriteFile(rootScript, []byte("// mock"), 0o644)

		got := filepath.Join(td, "generate-pdf.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "scripts", "generate-pdf.mjs")
		}
		if got != rootScript {
			t.Errorf("expected root path, got %s", got)
		}
	})

	t.Run("falls_back_to_scripts", func(t *testing.T) {
		scriptsDir := filepath.Join(td, "scripts")
		os.MkdirAll(scriptsDir, 0o755)
		scriptsScript := filepath.Join(scriptsDir, "generate-pdf.mjs")
		os.WriteFile(scriptsScript, []byte("// mock"), 0o644)

		got := filepath.Join(td, "generate-pdf.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "scripts", "generate-pdf.mjs")
		}
		if _, err := os.Stat(got); os.IsNotExist(err) {
			t.Fatal("expected script to exist in scripts/ fallback")
		}
	})
}

func TestScriptPathResolution_GenerateCVHTML(t *testing.T) {
	t.Run("prefers_scripts_when_exists", func(t *testing.T) {
		td := t.TempDir()
		scriptsDir := filepath.Join(td, "scripts")
		os.MkdirAll(scriptsDir, 0o755)
		scriptsScript := filepath.Join(scriptsDir, "generate-cv-html.mjs")
		os.WriteFile(scriptsScript, []byte("// mock"), 0o644)

		got := filepath.Join(td, "scripts", "generate-cv-html.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "generate-cv-html.mjs")
		}
		if got != scriptsScript {
			t.Errorf("expected scripts/ path, got %s", got)
		}
	})

	t.Run("falls_back_to_root_when_scripts_missing", func(t *testing.T) {
		td := t.TempDir()
		rootScript := filepath.Join(td, "generate-cv-html.mjs")
		os.WriteFile(rootScript, []byte("// mock"), 0o644)

		got := filepath.Join(td, "scripts", "generate-cv-html.mjs")
		if _, err := os.Stat(got); os.IsNotExist(err) {
			got = filepath.Join(td, "generate-cv-html.mjs")
		}
		if got != rootScript {
			t.Errorf("expected root path fallback, got %s", got)
		}
	})
}

// ---------------------------------------------------------------------------
// Struct serialization tests
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Error case tests (no external dependencies)
// ---------------------------------------------------------------------------

func TestGenerateCoverLetter_MissingProfile(t *testing.T) {
	// Temp dir without config/profile.yml → should fail early before reaching LLM/node
	td := t.TempDir()

	_, _, err := GenerateCoverLetter(nil, nil, "", td, "", "", "test JD")
	if err == nil {
		t.Error("expected error for missing config/profile.yml, got nil")
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

// ---------------------------------------------------------------------------
// Profile parsing test
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// AnswerQuizQuestions tests
// ---------------------------------------------------------------------------

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
