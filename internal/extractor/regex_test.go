package extractor

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func readFixture(t *testing.T, filename string) string {
	t.Helper()
	path := filepath.Join("testdata", filename)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture %q: %v", filename, err)
	}
	return string(b)
}

func TestExtractCSRF(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			name:    "Standard csrf input",
			html:    `<div><input type="hidden" name="csrfmiddlewaretoken" value="abc123xyz"></div>`,
			want:    "abc123xyz",
			wantErr: false,
		},
		{
			name:    "Csrf input with line breaks or extra spaces",
			html:    `<input  class="form-control"  name="csrfmiddlewaretoken"  value="token456" />`,
			want:    "token456",
			wantErr: false,
		},
		{
			name:    "Missing CSRF token",
			html:    `<div><input name="other_field" value="test"></div>`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractCSRF(tt.html)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractCSRF() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ExtractCSRF() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractJobs(t *testing.T) {
	tests := []struct {
		name    string
		html    string
		want    []Job
	}{
		{
			name: "From fixture",
			html: readFixture(t, "job_listing.html"),
			want: []Job{
				{
					ID:    "33333",
					Slug:  "33333-listing-job-1",
					Title: "Listing Job 1",
					URL:   "https://djinni.co/jobs/33333-listing-job-1/",
				},
				{
					ID:    "44444",
					Slug:  "44444-listing-job-2",
					Title: "Listing Job 2",
					URL:   "https://djinni.co/jobs/44444-listing-job-2/",
				},
			},
		},
		{
			name: "Unicode and duplicate slugs",
			html: `<div>
				<a href="/jobs/10101-golang-rozrobnik">Golang розробник</a>
				<a href="/jobs/10101-golang-rozrobnik">Golang розробник duplicate</a>
				<a href="/jobs/20202-kiiv-go">Київ Go Developer</a>
			</div>`,
			want: []Job{
				{
					ID:    "10101",
					Slug:  "10101-golang-rozrobnik",
					Title: "Golang розробник",
					URL:   "https://djinni.co/jobs/10101-golang-rozrobnik/",
				},
				{
					ID:    "20202",
					Slug:  "20202-kiiv-go",
					Title: "Київ Go Developer",
					URL:   "https://djinni.co/jobs/20202-kiiv-go/",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractJobs(tt.html)
			if err != nil {
				t.Fatalf("ExtractJobs failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExtractJobs() got = %+v, want = %+v", got, tt.want)
			}
		})
	}
}

func TestExtractDashboardJobs(t *testing.T) {
	html := readFixture(t, "dashboard_page.html")

	want := []Job{
		{
			ID:    "11111",
			Slug:  "11111-dashboard-job-1",
			Title: "Dashboard Job 1",
			URL:   "https://djinni.co/jobs/11111-dashboard-job-1/",
		},
		{
			ID:    "22222",
			Slug:  "22222-dashboard-job-2",
			Title: "Dashboard Job 2 · Remote",
			URL:   "https://djinni.co/jobs/22222-dashboard-job-2/",
		},
	}

	got, err := ExtractDashboardJobs(html)
	if err != nil {
		t.Fatalf("ExtractDashboardJobs failed: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractDashboardJobs() got = %+v, want = %+v", got, want)
	}
}

func TestExtractJobDetails(t *testing.T) {
	t.Run("JSON-LD Match", func(t *testing.T) {
		html := readFixture(t, "job_details_with_ld.html")

		got, err := ExtractJobDetails(html)
		if err != nil {
			t.Fatalf("ExtractJobDetails failed: %v", err)
		}

		if got.Title != "Software Engineer · JSON-LD" {
			t.Errorf("Title got = %q, want = %q", got.Title, "Software Engineer · JSON-LD")
		}
		if got.Company != "LD Company  Inc" {
			t.Errorf("Company got = %q, want = %q", got.Company, "LD Company  Inc")
		}
		if got.Description != "We are looking for a dev.\nНаші очікування:\n- Go\n- SQL" {
			t.Errorf("Description got = %q", got.Description)
		}
		if got.Requirements != "- Go\n- SQL" {
			t.Errorf("Requirements got = %q", got.Requirements)
		}
	})

	t.Run("Fallback HTML matching and requirement slicing", func(t *testing.T) {
		html := readFixture(t, "job_details_no_ld.html")

		got, err := ExtractJobDetails(html)
		if err != nil {
			t.Fatalf("ExtractJobDetails failed: %v", err)
		}

		if got.Title != "Software Engineer · HTML Fallback" {
			t.Errorf("Title got = %q", got.Title)
		}
		if got.Company != "HTML Cool Company" {
			t.Errorf("Company got = %q", got.Company)
		}
		if got.Description != "HTML description block.\nОчікуємо від вас:\n3+ years Go\nKubernetes" {
			t.Errorf("Description got = %q", got.Description)
		}
		if got.Requirements != "3+ years Go\nKubernetes" {
			t.Errorf("Requirements got = %q", got.Requirements)
		}
	})

	t.Run("Job with no apply button", func(t *testing.T) {
		html := readFixture(t, "job_no_apply_button.html")
		got, err := ExtractJobDetails(html)
		if err != nil {
			t.Fatalf("ExtractJobDetails failed: %v", err)
		}
		if got.Title != "Software Engineer · No Apply" {
			t.Errorf("Title got = %q", got.Title)
		}
		if got.Company != "No Apply Company" {
			t.Errorf("Company got = %q", got.Company)
		}
	})

	t.Run("Clean description tag variants", func(t *testing.T) {
		html := `
		<!DOCTYPE html>
		<html>
		<body>
		  <h1> Clean Developer </h1>
		  <div class="company_name">
			<a href="/jobs/company/">Clean Co</a>
		  </div>
		  <div class="job-post__description">
			<div>Part 1</div>
			<span>Part 2</span>
			<br/>
			<p>Part 3</p>
			<strong>Requirement:</strong>
			<ul><li>Go</li></ul>
		  </div>
		</body>
		</html>`
		got, err := ExtractJobDetails(html)
		if err != nil {
			t.Fatalf("ExtractJobDetails failed: %v", err)
		}
		if got.Title != "Clean Developer" {
			t.Errorf("Title got = %q", got.Title)
		}
		// Based on HTML parsing, the description strips tags appropriately
		if got.Description == "" {
			t.Errorf("Description should not be empty")
		}
	})
}

func TestExtractJobDetailsV2(t *testing.T) {
	html := `
	<!DOCTYPE html>
	<html>
	<body>
		<h1>Data Scientist (LLM, LangChain, RAG)</h1>
		<div class="company_name"><a href="/jobs/company-dataforest/">Dataforest</a></div>
		<div class="job-post__description">We need an expert.</div>
		
		<form>
			<input type="hidden" name="quiz_id" value="49880">
			
			<div>
				<label for="ans1">What is your Python experience?</label>
				<input type="text" name="answer_131741" id="ans1">
			</div>
			
			<label>
				What is your RAG experience?
				<textarea name="answer_131742"></textarea>
			</label>
			
			<div>
				<label>Expected salary?</label>
				<select name="answer_131743">
					<option value="1">1000</option>
				</select>
			</div>
		</form>
	</body>
	</html>`

	got, err := ExtractJobDetailsV2(html)
	if err != nil {
		t.Fatalf("ExtractJobDetailsV2 failed: %v", err)
	}

	if got.Title != "Data Scientist (LLM, LangChain, RAG)" {
		t.Errorf("Title = %q, want Data Scientist (LLM, LangChain, RAG)", got.Title)
	}
	if got.Company != "Dataforest" {
		t.Errorf("Company = %q, want Dataforest", got.Company)
	}
	if got.QuizID != "49880" {
		t.Errorf("QuizID = %q, want 49880", got.QuizID)
	}

	expectedQuestions := []QuizQuestion{
		{Name: "answer_131741", Text: "What is your Python experience?"},
		{Name: "answer_131742", Text: "What is your RAG experience?"},
		{Name: "answer_131743", Text: "Expected salary?"},
	}

	if len(got.QuizQuestions) != len(expectedQuestions) {
		t.Fatalf("len(QuizQuestions) = %d, want %d", len(got.QuizQuestions), len(expectedQuestions))
	}

	for i, eq := range expectedQuestions {
		gq := got.QuizQuestions[i]
		if gq.Name != eq.Name {
			t.Errorf("Question %d: Name = %q, want %q", i, gq.Name, eq.Name)
		}
		if gq.Text != eq.Text {
			t.Errorf("Question %d: Text = %q, want %q", i, gq.Text, eq.Text)
		}
	}
}
