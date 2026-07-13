package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/config"
)

func TestGetDashboardJobs(t *testing.T) {
	mockHtml := `
		<div class="job-list">
			<a class="card" href="/jobs/12345-go-dev/?ref=for_me">
				<h2 class="job-item__position">Go Developer</h2>
			</a>
		</div>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/my/dashboard/" {
			if r.Header.Get("Referer") == "" {
				t.Errorf("expected Referer header to be set")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHtml))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		SessionID: "mock-session",
		CSRFToken: "mock-csrf",
	}
	dc := client.NewDjinniClient(cfg)
	dc.Client.SetBaseURL(server.URL)

	jobs, err := GetDashboardJobs(dc, 1)
	if err != nil {
		t.Fatalf("GetDashboardJobs failed: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].ID != "12345" || jobs[0].Title != "Go Developer" {
		t.Errorf("unexpected job details: %+v", jobs[0])
	}
}

func TestSearchJobs(t *testing.T) {
	mockHtml := `
		<div>
			<a class="job-link" href="/jobs/67890-python-dev/">Python Developer</a>
		</div>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/" {
			if r.URL.Query().Get("primary_keyword") != "python" {
				t.Errorf("expected primary_keyword query parameter 'python', got %q", r.URL.Query().Get("primary_keyword"))
			}
			if r.Header.Get("Referer") == "" {
				t.Errorf("expected Referer header to be set")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockHtml))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		SessionID: "mock-session",
		CSRFToken: "mock-csrf",
	}
	dc := client.NewDjinniClient(cfg)
	dc.Client.SetBaseURL(server.URL)

	jobs, err := SearchJobs(dc, map[string]string{"primary_keyword": "python"})
	if err != nil {
		t.Fatalf("SearchJobs failed: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	if jobs[0].ID != "67890" || jobs[0].Title != "Python Developer" {
		t.Errorf("unexpected job details: %+v", jobs[0])
	}
}

func TestApplyToJob(t *testing.T) {
	t.Run("Apply without CV", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/jobs/111-go-dev/" {
				if r.Header.Get("X-CSRFToken") != "mock-csrf" {
					t.Errorf("missing or invalid X-CSRFToken header")
				}
				err := r.ParseMultipartForm(1024 * 1024)
				if err != nil {
					t.Errorf("failed to parse multipart form: %v", err)
				}
				if r.FormValue("apply") != "true" {
					t.Errorf("expected apply 'true', got %q", r.FormValue("apply"))
				}
				if r.FormValue("message") != "I want to apply" {
					t.Errorf("expected message 'I want to apply', got %q", r.FormValue("message"))
				}
				if r.FormValue("csrfmiddlewaretoken") != "mock-csrf" {
					t.Errorf("expected csrfmiddlewaretoken 'mock-csrf', got %q", r.FormValue("csrfmiddlewaretoken"))
				}
				if _, _, fileErr := r.FormFile("cv_file"); fileErr == nil {
					t.Errorf("expected no cv_file to be present")
				}
				// Redirect to /jobs/111-go-dev/?applied=ok
				w.Header().Set("Location", "/jobs/111-go-dev/?applied=ok")
				w.WriteHeader(http.StatusFound)
				return
			}
			if r.Method == http.MethodGet && r.URL.Path == "/jobs/111-go-dev/" {
				if r.URL.Query().Get("applied") != "ok" {
					t.Errorf("expected applied=ok query parameter in redirect target")
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Application Success page"))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		cfg := &config.Config{
			SessionID: "mock-session",
			CSRFToken: "mock-csrf",
		}
		dc := client.NewDjinniClient(cfg)
		dc.Client.SetBaseURL(server.URL)

		resp, err := ApplyToJob(dc, "111-go-dev", "I want to apply", "", nil, nil)
		if err != nil {
			t.Fatalf("ApplyToJob failed: %v", err)
		}
		if resp != "Application Success" {
			t.Errorf("expected 'Application Success', got %q", resp)
		}
	})

	t.Run("Apply with CV", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/jobs/222-python-dev/" {
				if r.Header.Get("X-CSRFToken") != "mock-csrf" {
					t.Errorf("missing or invalid X-CSRFToken header")
				}
				err := r.ParseMultipartForm(1024 * 1024)
				if err != nil {
					t.Errorf("failed to parse multipart form: %v", err)
				}
				if r.FormValue("apply") != "true" {
					t.Errorf("expected apply 'true', got %q", r.FormValue("apply"))
				}
				if r.FormValue("message") != "I want to apply with CV" {
					t.Errorf("expected message 'I want to apply with CV', got %q", r.FormValue("message"))
				}
				file, header, fileErr := r.FormFile("cv_file")
				if fileErr != nil {
					t.Fatalf("expected cv_file to be present: %v", fileErr)
				}
				defer file.Close()

				if header.Filename != "resume.pdf" {
					t.Errorf("expected filename 'resume.pdf', got %q", header.Filename)
				}

				content, readErr := io.ReadAll(file)
				if readErr != nil {
					t.Fatalf("failed to read file: %v", readErr)
				}
				if string(content) != "PDF_CONTENT" {
					t.Errorf("expected file content 'PDF_CONTENT', got %q", string(content))
				}

				w.Header().Set("Location", "/jobs/222-python-dev/?applied=ok")
				w.WriteHeader(http.StatusFound)
				return
			}
			if r.Method == http.MethodGet && r.URL.Path == "/jobs/222-python-dev/" {
				if r.URL.Query().Get("applied") != "ok" {
					t.Errorf("expected applied=ok query parameter in redirect target")
				}
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Application Success page with CV"))
				return
			}
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		cfg := &config.Config{
			SessionID: "mock-session",
			CSRFToken: "mock-csrf",
		}
		dc := client.NewDjinniClient(cfg)
		dc.Client.SetBaseURL(server.URL)

		resp, err := ApplyToJob(dc, "222-python-dev", "I want to apply with CV", "resume.pdf", []byte("PDF_CONTENT"), nil)
		if err != nil {
			t.Fatalf("ApplyToJob failed: %v", err)
		}
		if resp != "Application Success" {
			t.Errorf("expected 'Application Success', got %q", resp)
		}
	})
}

func TestGetJobDetails(t *testing.T) {
	t.Run("Valid Job", func(t *testing.T) {
		mockHtml := `
			<script type="application/ld+json">
			{
				"@type": "JobPosting",
				"title": "Senior AI Engineer",
				"hiringOrganization": {"name": "Tech Corp"},
				"description": "Great job"
			}
			</script>
			<button class="js-inbox-toggle-reply-form">Apply</button>
		`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs/123-senior-ai-engineer/" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(mockHtml))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &config.Config{SessionID: "mock-session", CSRFToken: "mock-csrf"}
		dc := client.NewDjinniClient(cfg)
		dc.Client.SetBaseURL(server.URL)

		job, err := GetJobDetails(dc, "123-senior-ai-engineer")
		if err != nil {
			t.Fatalf("GetJobDetails failed: %v", err)
		}
		if job == nil {
			t.Fatal("expected job details, got nil")
		}
		if job.ID != "123" {
			t.Errorf("expected ID '123', got %q", job.ID)
		}
		if job.Title != "Senior AI Engineer" {
			t.Errorf("expected Title 'Senior AI Engineer', got %q", job.Title)
		}
		if job.Company != "Tech Corp" {
			t.Errorf("expected Company 'Tech Corp', got %q", job.Company)
		}
	})

	t.Run("Missing Apply Button", func(t *testing.T) {
		mockHtml := `
			<script type="application/ld+json">
			{
				"@type": "JobPosting",
				"title": "Senior AI Engineer",
				"hiringOrganization": {"name": "Tech Corp"},
				"description": "Great job"
			}
			</script>
			<!-- Notice the missing apply button -->
		`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/jobs/456-senior-ai-engineer/" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(mockHtml))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &config.Config{SessionID: "mock-session", CSRFToken: "mock-csrf"}
		dc := client.NewDjinniClient(cfg)
		dc.Client.SetBaseURL(server.URL)

		_, err := GetJobDetails(dc, "456-senior-ai-engineer")
		if err == nil {
			t.Fatal("expected error due to missing apply button, got nil")
		}
		if err.Error() != "job is strictly blocked by Djinni requirements or already applied (missing apply button, error=cant_apply)" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("Empty Slug", func(t *testing.T) {
		dc := client.NewDjinniClient(&config.Config{})
		_, err := GetJobDetails(dc, "")
		if err == nil {
			t.Fatal("expected error for empty slug")
		}
	})
}

func TestSimilarJobsHTMX(t *testing.T) {
	// Skip if no credentials (e.g. CSRFToken or SessionID)
	// For testing, we just check if it's set in environment or we skip.
	// Actually we just make a dummy server like others.
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/123/similar-jobs/" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<div class="similar-jobs">...</div>`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		SessionID: "mock-session",
		CSRFToken: "mock-csrf",
	}
	dc := client.NewDjinniClient(cfg)
	dc.Client.SetBaseURL(server.URL)

	// Since we are just investigating the endpoint for HTMX, we do a basic request
	resp, err := dc.Client.R().
		SetHeader("HX-Request", "true").
		Get(server.URL + "/jobs/123/similar-jobs/")

	if err != nil {
		t.Fatalf("HTMX request failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}
}
