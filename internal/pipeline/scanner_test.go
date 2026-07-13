package pipeline

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
)

func TestScanner_LoadTitleFilter(t *testing.T) {
	tmp := t.TempDir()

	portalsContent := `
title_filter:
  positive:
    - "AI"
    - "LLM"
  negative:
    - "Crypto"
`

	if err := os.WriteFile(filepath.Join(tmp, "portals.yml"), []byte(portalsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	filter, err := LoadTitleFilter(tmp)
	if err != nil {
		t.Fatalf("LoadTitleFilter failed: %v", err)
	}

	cases := []struct {
		title string
		want  bool
	}{
		{"AI Engineer", true},
		{"LLM Developer", true},
		{"Crypto AI Specialist", false}, // blocks negative
		{"Go Developer", false},        // no positive keywords
	}

	for _, c := range cases {
		got := filter(c.title)
		if got != c.want {
			t.Errorf("filter(%q) = %t, want %t", c.title, got, c.want)
		}
	}
}

func TestScanDjinni(t *testing.T) {
	// Setup a mock server that simulates Djinni's dashboard and search pages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my/dashboard/" {
			w.WriteHeader(http.StatusOK)
			page := r.URL.Query().Get("page")
			if page == "1" || page == "" {
				w.Write([]byte(`
					<div class="job-list">
						<a class="card" href="/jobs/111-ai-developer/?ref=for_me"><h2 class="job-item__position">AI Developer</h2></a>
						<a class="card" href="/jobs/222-boring-dev/?ref=for_me"><h2 class="job-item__position">Boring Dev</h2></a>
					</div>
				`))
			} else if page == "2" {
				w.Write([]byte(`
					<div class="job-list">
						<a class="card" href="/jobs/333-llm-engineer/?ref=for_me"><h2 class="job-item__position">LLM Engineer</h2></a>
					</div>
				`))
			} else {
				// empty for pages > 2
				w.Write([]byte(`<div></div>`))
			}
			return
		}
		
		if r.URL.Path == "/jobs/" {
			// Search endpoint
			title := r.URL.Query().Get("title")
			if title == "AI" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`
					<div>
						<a class="job-link" href="/jobs/111-ai-developer/">AI Developer</a>
						<a class="card" href="/jobs/444-ai-researcher/?ref=for_me"><h2 class="job-item__position">AI Researcher</h2></a>
					</div>
				`))
			} else if title == "LLM" {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server Error"))
				return
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`<div></div>`))
			}
			return
		}
		
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Temporary directory for context (portals.yml and dedup db)
	tmp := t.TempDir()

	portalsContent := `
title_filter:
  positive:
    - "AI"
    - "LLM"
  negative:
    - "Crypto"
`
	if err := os.WriteFile(filepath.Join(tmp, "portals.yml"), []byte(portalsContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a mock history file to test deduplication
	os.MkdirAll(filepath.Join(tmp, "data"), 0755)
	os.WriteFile(filepath.Join(tmp, "data", "scan-history.tsv"), []byte("url\nhttps://djinni.co/jobs/444-ai-researcher/\n"), 0644)

	dedup, err := LoadDedup(tmp)
	if err != nil {
		t.Fatalf("Failed to create dedup: %v", err)
	}

	// Create client pointing to mock server
	cfg := &config.Config{
		SessionID: "mock-session",
		CSRFToken: "mock-csrf",
	}
	dc := client.NewDjinniClient(cfg)
	dc.Client.SetBaseURL(server.URL) // Set the mock server as the base URL

	jobs, err := ScanDjinni(tmp, dc, dedup)
	if err != nil {
		t.Fatalf("ScanDjinni failed: %v", err)
	}

	// We expect:
	// 111-ai-developer (from dashboard, matches "AI")
	// 222-boring-dev (filtered out, does not match "AI" or "LLM")
	// 333-llm-engineer (from dashboard page 2, matches "LLM")
	// 444-ai-researcher (from search "AI", but filtered out by Dedup)
	
	if len(jobs) != 2 {
		t.Fatalf("Expected 2 jobs, got %d", len(jobs))
	}

	jobSlugs := make(map[string]bool)
	for _, j := range jobs {
		jobSlugs[j.Slug] = true
	}

	if !jobSlugs["111-ai-developer"] {
		t.Errorf("Expected 111-ai-developer to be found")
	}
	if !jobSlugs["333-llm-engineer"] {
		t.Errorf("Expected 333-llm-engineer to be found")
	}
	if jobSlugs["222-boring-dev"] {
		t.Errorf("Did not expect 222-boring-dev (filtered by title)")
	}
	if jobSlugs["444-ai-researcher"] {
		t.Errorf("Did not expect 444-ai-researcher (filtered by dedup)")
	}
}
