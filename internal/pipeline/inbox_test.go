package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindReport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "reports-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	reportsDir := filepath.Join(tempDir, "reports")
	if err := os.Mkdir(reportsDir, 0755); err != nil {
		t.Fatalf("failed to create reports dir: %v", err)
	}

	testCases := []struct {
		name        string
		sender      string
		files       map[string]string
		expectFound bool
		expectData  string
	}{
		{
			name:   "found exact company slug",
			sender: "Google / Recruiter Name",
			files: map[string]string{
				"report-google-123.md": "google report content",
				"report-meta-456.md":   "meta report content",
			},
			expectFound: true,
			expectData:  "google report content",
		},
		{
			name:   "company name too short",
			sender: "Go",
			files: map[string]string{
				"report-google-123.md": "google report content",
			},
			expectFound: false,
		},
		{
			name:   "not found",
			sender: "Netflix / Recruiter",
			files: map[string]string{
				"report-google-123.md": "google report content",
			},
			expectFound: false,
		},
		{
			name:   "complex sender name",
			sender: "Awesome-Tech, LLC / Recruiter",
			files: map[string]string{
				"report-awesome-tech-llc-1.md": "awesome tech report",
			},
			expectFound: true,
			expectData:  "awesome tech report",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clean reports dir
			entries, _ := os.ReadDir(reportsDir)
			for _, e := range entries {
				os.Remove(filepath.Join(reportsDir, e.Name()))
			}

			// Create files
			for filename, content := range tc.files {
				err := os.WriteFile(filepath.Join(reportsDir, filename), []byte(content), 0644)
				if err != nil {
					t.Fatalf("failed to write test file: %v", err)
				}
			}

			content, err := findReport(tempDir, tc.sender)
			if tc.expectFound {
				if err != nil {
					t.Errorf("expected to find report, got error: %v", err)
				}
				if content != tc.expectData {
					t.Errorf("expected content %q, got %q", tc.expectData, content)
				}
			} else {
				if err == nil {
					t.Errorf("expected error, but found report")
				}
			}
		})
	}
}

func TestLoadSeenIDs(t *testing.T) {
	tempFile, err := os.CreateTemp("", "inbox-log-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	content := "2023-10-01 12:00:00\tdialog-123\tGoogle / Recruiter\tReply text 1\n" +
		"2023-10-01 12:05:00\tdialog-456\tMeta / Recruiter\tReply text 2\n" +
		"\n" + // empty line
		"invalid line without tabs\n" +
		"2023-10-01 12:10:00\tdialog-789\tApple / Recruiter\tReply text 3\n"

	if _, err := tempFile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tempFile.Close()

	seen, err := loadSeenIDs(tempFile.Name())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedIDs := []string{"dialog-123", "dialog-456", "dialog-789"}
	for _, id := range expectedIDs {
		if !seen[id] {
			t.Errorf("expected ID %s to be seen", id)
		}
	}

	if len(seen) != 3 {
		t.Errorf("expected exactly 3 seen IDs, got %d", len(seen))
	}
}

func TestLoadSeenIDs_NotExist(t *testing.T) {
	seen, err := loadSeenIDs("/path/that/does/not/exist/inbox.log")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(seen) != 0 {
		t.Errorf("expected empty map, got %v", seen)
	}
}
