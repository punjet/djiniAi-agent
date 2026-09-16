package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://djinni.co/jobs/123-slug?query=1#hash", "https://djinni.co/jobs/123-slug"},
		{"https://djinni.co/jobs/456", "https://djinni.co/jobs/456"},
		{"/jobs/123?query=1", "/jobs/123"},
		{"invalid-url?query=1", "invalid-url"},
	}

	for _, tt := range tests {
		result := cleanURL(tt.input)
		if result != tt.expected {
			t.Errorf("cleanURL(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello, World!", "hello world"},
		{"   Multiple    Spaces  ", "multiple spaces"},
		{"C++ Developer", "c developer"},
		{"Тест-Рядок №1", "тестрядок 1"},
	}

	for _, tt := range tests {
		result := normalizeString(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeString(%q) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGetSignifcantWords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"hello world test", []string{"hello", "world", "test"}},
		{"a bc def ghij", []string{"ghij"}},
		{"тест рядок для перевірки", []string{"тест", "рядок", "перевірки"}},
	}

	for _, tt := range tests {
		result := getSignifcantWords(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("getSignifcantWords(%q) len = %d; want %d", tt.input, len(result), len(tt.expected))
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("getSignifcantWords(%q)[%d] = %q; want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestDedup_IsNew(t *testing.T) {
	d := &Dedup{
		seenURLs:         map[string]bool{"https://example.com/job/1": true},
		seenCompanyRoles: map[string]bool{"google::software engineer": true},
	}

	tests := []struct {
		name     string
		url      string
		company  string
		role     string
		expected bool
	}{
		{"new url and role", "https://example.com/job/2", "meta", "data engineer", true},
		{"seen url", "https://example.com/job/1", "meta", "data engineer", false},
		{"seen company and exact role", "https://example.com/job/3", "Google", "Software Engineer", false},
		{"seen company and fuzzy role", "https://example.com/job/4", "Google", "Senior Software Engineer (Backend)", false},
		{"seen company but totally different role", "https://example.com/job/5", "Google", "Product Manager", true},
		{"empty company/role", "https://example.com/job/6", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.IsNew(tt.url, tt.company, tt.role)
			if result != tt.expected {
				t.Errorf("IsNew() = %v; want %v", result, tt.expected)
			}
		})
	}
}

func TestLoadDedup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dedup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	historyContent := "url\tfirst_seen\tportal\ttitle\tcompany\tstatus\n" +
		"https://djinni.co/jobs/1\t2023-10-01\tDjinni\tEng\tGoogle\tadded\n"

	appsContent := "| # | Date | Company | Role | URL |\n" +
		"|---|---|---|---|---|\n" +
		"| 1 | 2023-10-02 | Meta | Data Engineer | https://djinni.co/jobs/2 |\n"

	os.WriteFile(filepath.Join(dataDir, "scan-history.tsv"), []byte(historyContent), 0644)
	os.WriteFile(filepath.Join(dataDir, "applications.md"), []byte(appsContent), 0644)

	d, err := LoadDedup(tempDir)
	if err != nil {
		t.Fatalf("LoadDedup error: %v", err)
	}

	if !d.seenURLs["https://djinni.co/jobs/1"] {
		t.Errorf("expected URL 1 to be seen")
	}
	if !d.seenURLs["https://djinni.co/jobs/2"] {
		t.Errorf("expected URL 2 to be seen")
	}
	if !d.seenCompanyRoles["meta::data engineer"] {
		t.Errorf("expected meta::data engineer to be seen")
	}
}

func TestAppendToScanHistory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "dedup-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	AppendToScanHistory(tempDir, "https://example.com/job/3", "Djinni", "Go Developer", "Apple")

	historyPath := filepath.Join(tempDir, "data", "scan-history.tsv")
	content, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("failed to read history file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "url\tfirst_seen") {
		t.Errorf("missing header in history file")
	}
	if !strings.Contains(contentStr, "https://example.com/job/3") {
		t.Errorf("missing appended job url")
	}
	if !strings.Contains(contentStr, "Go Developer") {
		t.Errorf("missing title in history file")
	}
}
