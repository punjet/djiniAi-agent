package covergen

import (
	"context"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFolioCV(t *testing.T) {
	// Read template
	templatePath := filepath.Join("../../career-ops/templates/cv-template.html")
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("failed to read template: %v", err)
	}

	// Mock data
	tmplData := map[string]interface{}{
		"LANG":                   "en",
		"NAME":                   "Alex Engineer",
		"EMAIL":                  "alex@example.com",
		"LINKEDIN_URL":           "https://linkedin.com/in/alex",
		"LINKEDIN_DISPLAY":       "linkedin.com/in/alex",
		"PORTFOLIO_URL":          "https://github.com/alex",
		"PORTFOLIO_DISPLAY":      "github.com/alex",
		"LOCATION":               "Remote",
		"PAGE_WIDTH":             "900px",
		"SECTION_SUMMARY":        "Professional Summary",
		"SUMMARY_TEXT":           "Experienced software engineer specializing in backend development with Go. Proven track record of delivering scalable systems and working effectively in remote environments.",
		"SECTION_COMPETENCIES":   "Core Competencies",
		"COMPETENCIES":           template.HTML("<span class=\"competency-tag\">Golang</span> <span class=\"competency-tag\">PostgreSQL</span> <span class=\"competency-tag\">System Architecture</span>"),
		"SECTION_EXPERIENCE":     "Experience",
		"EXPERIENCE":             template.HTML("<div class=\"job\"><div class=\"job-header\"><span class=\"job-company\">Tech Solutions Inc</span> <span class=\"job-period\">2020 - Present</span></div><div class=\"job-role\">Senior Backend Engineer</div><ul><li>Led the migration to microservices, improving uptime by 99.9%.</li><li>Mentored junior engineers and conducted code reviews.</li></ul></div>"),
		"SECTION_PROJECTS":       "Projects",
		"PROJECTS":               template.HTML("<div class=\"project\"><div class=\"project-title\">Open Source PDF Generator</div><div class=\"project-desc\">Built a high-performance PDF generator in Go handling thousands of documents per minute.</div></div>"),
		"SECTION_EDUCATION":      "Education",
		"EDUCATION":              template.HTML("<div class=\"edu-item\"><div class=\"edu-header\"><span class=\"edu-title\">B.S. Computer Science</span> <span class=\"edu-org\">University of Technology</span> <span class=\"edu-year\">2016 - 2020</span></div></div>"),
		"SECTION_CERTIFICATIONS": "Certifications",
		"CERTIFICATIONS":         template.HTML("<div class=\"cert-item\"><span class=\"cert-title\">AWS Certified Solutions Architect</span> <span class=\"cert-org\">Amazon Web Services</span> <span class=\"cert-year\">2023</span></div>"),
		"SECTION_SKILLS":         "Technical Skills",
		"SKILLS":                 template.HTML("<span class=\"skill-item\"><span class=\"skill-category\">Languages:</span> Go, Python, JavaScript</span><br/><span class=\"skill-item\"><span class=\"skill-category\">Tools:</span> Docker, Kubernetes, Git</span>"),
	}

	tmpl, err := template.New("cv").Parse(preprocessTemplate(string(tmplBytes)))
	if err != nil {
		t.Fatalf("failed to parse CV template: %v", err)
	}

	var htmlBuf strings.Builder
	if err := tmpl.Execute(&htmlBuf, tmplData); err != nil {
		t.Fatalf("failed to execute CV template: %v", err)
	}

	htmlStr := htmlBuf.String()

	// Call Folio renderer directly
	pdfBytes, err := renderHTMLToPDFFolio(context.Background(), htmlStr)
	if err != nil {
		t.Fatalf("failed to render CV PDF: %v", err)
	}

	// Write out to the root directory
	outPath := filepath.Join("../../folio_test_output.pdf")
	err = os.WriteFile(outPath, pdfBytes, 0644)
	if err != nil {
		t.Fatalf("failed to write PDF: %v", err)
	}

	t.Logf("Successfully generated %s", outPath)
}
