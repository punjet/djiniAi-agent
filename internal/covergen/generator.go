package covergen

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/extractor"
	"djinni-bot-go/internal/llm"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	Candidate struct {
		FullName string `yaml:"full_name"`
		Email    string `yaml:"email"`
		Phone    string `yaml:"phone"`
		Location string `yaml:"location"`
		Linkedin string `yaml:"linkedin"`
		Github   string `yaml:"github"`
	} `yaml:"candidate"`
	TargetRoles struct {
		Primary []string `yaml:"primary"`
	} `yaml:"target_roles"`
}

type Payload struct {
	Candidate  map[string]interface{} `json:"candidate"`
	Letter     map[string]interface{} `json:"letter"`
	OutputPath string                 `json:"output_path,omitempty"`
}

type GeneratedLetter struct {
	Letter        map[string]interface{} `json:"letter"`
	DjinniMessage string                 `json:"djinni_message"`
}

// GenerateCoverLetter calls the LLM to draft a cover letter and then invokes
// node generate-cover-letter.mjs to output a PDF file.
// Returns the PDF bytes, the Djinni message text, and any error.
func GenerateCoverLetter(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir string, company string, role string, jdText string) ([]byte, string, error) {
	// 1. Load profile
	profilePath := filepath.Join(contextDir, "config", "profile.yml")
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read profile.yml: %w", err)
	}

	var prof Profile
	if err := yaml.Unmarshal(profileData, &prof); err != nil {
		return nil, "", fmt.Errorf("failed to parse profile.yml: %w", err)
	}

	// 2. Load CV
	cvPath := filepath.Join(contextDir, "cv.md")
	cvData, err := os.ReadFile(cvPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read cv.md: %w", err)
	}

	// 3. Setup LLM provider
	provider, err := llm.NewProvider(cfg, engine)
	if err != nil {
		return nil, "", err
	}

	systemPrompt := `You are an expert technical resume writer. Write a highly tailored cover letter and a short Djinni message hook for a candidate applying to a job.

Instructions:
Draft a cover letter and short message following these guidelines (inspired by Santiago's career-ops):
1. Greeting: tailored to the company or hiring manager.
2. Opening: clear state of application, role title, and immediate hook showing you understand their domain.
3. Profile Intro: 2-3 sentences matching the candidate's core narrative to the role.
4. Achievements: 2-3 achievements tailored to the role, with a 'lead' (what candidate did) and 'impact' (quantified results).
5. Problems Section: explain how candidate's superpowers solve their specific challenges.
6. Closing: selective, direct, confident.
7. Djinni Message: A short, concise hook (3-4 sentences) for the initial message. Do not make it generic. Highlight the candidate's core value match.
8. Language Rule: Write the Cover Letter and Djinni message in UKRAINIAN if the Job Description is in Ukrainian or Russian. Write in ENGLISH if the Job Description is in English. NEVER write in Russian under any circumstances.
9. NO MARKDOWN: Do not use ANY markdown formatting (like **, *, #) anywhere in your response. The output must be pure plain text.

You MUST respond with a single JSON object (no markdown wrappers like ` + "`" + `json or comments) matching this schema exactly:
{
  "letter": {
    "role_title": "...",
    "company": "...",
    "city": "...",
    "greeting": "...",
    "opening": "...",
    "profile_intro": "...",
    "achievements": [
      { "lead": "...", "impact": "..." }
    ],
    "problems_section": "...",
    "closing": "..."
  },
  "djinni_message": "..."
}`

	userPrompt := fmt.Sprintf(`Candidate Context:
Name: %s
Email: %s
Phone: %s
Location: %s
Linkedin: %s
Github: %s
Resume content:
%s

Job Details:
Company: %s
Role: %s
JD Content:
%s`, prof.Candidate.FullName, prof.Candidate.Email, prof.Candidate.Phone, prof.Candidate.Location, prof.Candidate.Linkedin, prof.Candidate.Github, string(cvData), company, role, jdText)

	response, err := provider.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, "", fmt.Errorf("LLM cover letter generation failed: %w", err)
	}

	// Clean markdown wrappers if any
	cleanJSON := response
	if idx := strings.Index(cleanJSON, "{"); idx != -1 {
		cleanJSON = cleanJSON[idx:]
	}
	if idx := strings.LastIndex(cleanJSON, "}"); idx != -1 {
		cleanJSON = cleanJSON[:idx+1]
	}

	var genLetter GeneratedLetter
	if err := json.Unmarshal([]byte(cleanJSON), &genLetter); err != nil {
		return nil, "", fmt.Errorf("failed to parse LLM response JSON: %w (raw response: %q)", err, response)
	}

	genLetter.DjinniMessage = strings.ReplaceAll(genLetter.DjinniMessage, "**", "")
	genLetter.DjinniMessage = strings.ReplaceAll(genLetter.DjinniMessage, "*", "")

	// 4. Build payload for generate-cover-letter.mjs
	today := time.Now().Format("2006-01-02")
	candidateMap := map[string]interface{}{
		"name":     prof.Candidate.FullName,
		"email":    prof.Candidate.Email,
		"phone":    prof.Candidate.Phone,
		"location": prof.Candidate.Location,
		"linkedin": prof.Candidate.Linkedin,
		"github":   prof.Candidate.Github,
	}

	// Ensure fields in genLetter.Letter
	if genLetter.Letter == nil {
		genLetter.Letter = make(map[string]interface{})
	}
	genLetter.Letter["company"] = company
	genLetter.Letter["role_title"] = role
	genLetter.Letter["date"] = today
	if genLetter.Letter["city"] == nil || genLetter.Letter["city"] == "" {
		genLetter.Letter["city"] = prof.Candidate.Location
	}

	pdfOutFilename := fmt.Sprintf("%s-%s-cover.pdf", strings.ToLower(company), strings.ToLower(role))
	pdfOutFilename = regexp.MustCompile(`[^a-z0-9.-]+`).ReplaceAllString(pdfOutFilename, "-")
	pdfOutPath := filepath.Join(contextDir, "output", pdfOutFilename)

	payload := Payload{
		Candidate:  candidateMap,
		Letter:     genLetter.Letter,
		OutputPath: pdfOutPath,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, "", fmt.Errorf("failed to serialize payload: %w", err)
	}

	tmpPayloadFile := filepath.Join(os.TempDir(), fmt.Sprintf("payload-%d.json", time.Now().UnixNano()))
	if err := os.WriteFile(tmpPayloadFile, payloadBytes, 0o644); err != nil {
		return nil, "", fmt.Errorf("failed to write temporary payload: %w", err)
	}
	defer os.Remove(tmpPayloadFile)

	// 5. Invoke Node.js generator script
	// FRAGILE: External node process execution introduces environment dependencies and error handling challenges.
	// TODO: Port the PDF generation logic natively to Go, or use a robust worker queue / service architecture.
	// Resolve script path: prefer root of contextDir, fall back to scripts/ subdirectory.
	coverLetterScript := filepath.Join(contextDir, "generate-cover-letter.mjs")
	if _, err := os.Stat(coverLetterScript); os.IsNotExist(err) {
		coverLetterScript = filepath.Join(contextDir, "scripts", "generate-cover-letter.mjs")
	}
	coverLetterScript, _ = filepath.Abs(coverLetterScript)
	cmd := exec.CommandContext(ctx, "node", coverLetterScript, "--payload", tmpPayloadFile, "--out", pdfOutPath)
	cmd.Dir = contextDir
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr // keep stdout clean for caller logs, print script diagnostic to stderr
	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("node generate-cover-letter.mjs execution failed: %w", err)
	}

	// 6. Read PDF bytes
	pdfBytes, err := os.ReadFile(pdfOutPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read generated PDF: %w", err)
	}

	return pdfBytes, genLetter.DjinniMessage, nil
}

// AnswerQuizQuestions calls the LLM to answer recruiter quiz questions based on the candidate's CV and profile.
// It loads cv.md and config/profile.yml from contextDir, sends the questions to the LLM,
// and populates the Answer field on each question from the JSON response.
func AnswerQuizQuestions(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir string, questions []extractor.QuizQuestion, jdText, company, role string) ([]extractor.QuizQuestion, error) {
	// 1. Load profile
	profilePath := filepath.Join(contextDir, "config", "profile.yml")
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile.yml: %w", err)
	}

	var prof Profile
	if err := yaml.Unmarshal(profileData, &prof); err != nil {
		return nil, fmt.Errorf("failed to parse profile.yml: %w", err)
	}

	// 2. Load CV
	cvPath := filepath.Join(contextDir, "cv.md")
	cvData, err := os.ReadFile(cvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cv.md: %w", err)
	}

	// 3. Setup LLM provider
	provider, err := llm.NewProvider(cfg, engine)
	if err != nil {
		return nil, err
	}

	// 4. Build question list text
	var questionLines []string
	for i, q := range questions {
		questionLines = append(questionLines, fmt.Sprintf("  %d. [name=%s] %s", i+1, q.Name, q.Text))
	}
	questionsText := strings.Join(questionLines, "\n")

	systemPrompt := `You are Kyrylo Kirov, a Senior AI Automation Expert applying for a job. You have been sent a short quiz by the recruiter.

Your task: Answer each quiz question professionally, concisely, and truthfully based on your CV and profile context below.

Instructions:
1. Answer each question as if you are the candidate (Kyrylo Kirov). Use "I" / "me" where appropriate.
2. Be concise — 1-3 sentences per answer is usually enough.
3. Write each answer in the SAME LANGUAGE as the question itself (e.g. if the question is in English, answer in English; if in Ukrainian, answer in Ukrainian; if in Russian, answer in Russian).
4. Base your answers on the provided CV and profile. If the CV doesn't cover the topic, give a general but relevant answer.

You MUST respond with a single JSON object (no markdown wrappers like ` + "`" + `json or comments) matching this schema exactly:
{
  "answers": [
    {
      "name": "field_name_like_answer_12345",
      "answer": "your answer text here"
    }
  ]
}`

	userPrompt := fmt.Sprintf(`Candidate Profile:
Name: %s
Email: %s
Location: %s
LinkedIn: %s
GitHub: %s
Target Roles: %v

Resume/CV:
%s

Job Details:
Company: %s
Role: %s
Job Description:
%s

Recruiter Quiz Questions:
%s

Please answer the above questions and return them as JSON.`, prof.Candidate.FullName, prof.Candidate.Email, prof.Candidate.Location, prof.Candidate.Linkedin, prof.Candidate.Github, prof.TargetRoles.Primary, string(cvData), company, role, jdText, questionsText)

	response, err := provider.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM quiz answering failed: %w", err)
	}

	// 5. Parse JSON response — strip markdown wrappers if present
	cleanJSON := response
	if idx := strings.Index(cleanJSON, "{"); idx != -1 {
		cleanJSON = cleanJSON[idx:]
	}
	if idx := strings.LastIndex(cleanJSON, "}"); idx != -1 {
		cleanJSON = cleanJSON[:idx+1]
	}

	var result struct {
		Answers []struct {
			Name   string `json:"name"`
			Answer string `json:"answer"`
		} `json:"answers"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response JSON: %w (raw: %q)", err, response)
	}

	// 6. Map answers back to questions by name
	answerMap := make(map[string]string, len(result.Answers))
	for _, a := range result.Answers {
		answerMap[a.Name] = a.Answer
	}

	for i := range questions {
		if ans, ok := answerMap[questions[i].Name]; ok {
			questions[i].Answer = ans
		}
	}

	return questions, nil
}

// GenerateCustomCV generates a job-specific tailored CV PDF using scripts/generate-cv-html.mjs and generate-pdf.mjs.
// ValidateCVHTML checks if the generated CV HTML contains unresolved placeholders.
func ValidateCVHTML(htmlContent string) error {
	if strings.Contains(htmlContent, "{{") || strings.Contains(htmlContent, "}}") {
		return fmt.Errorf("generated CV contains unresolved template placeholders (found '{{' or '}}')")
	}
	return nil
}

func GenerateCustomCV(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir, jobURL, company, role, reportPath string) ([]byte, error) {
	// Create output dir if not exist
	outputDir := filepath.Join(contextDir, "output")
	_ = os.MkdirAll(outputDir, 0o755)

	// Step 1: Run generate-cv-html.mjs
	// FRAGILE: Shelling out to a node script for HTML generation depends on the local environment and path resolving.
	// TODO: Rewrite the CV HTML generation logic natively in Go.
	// Resolve script path: prefer scripts/generate-cv-html.mjs, fall back to root of contextDir.
	cvHtmlScript := filepath.Join(contextDir, "scripts", "generate-cv-html.mjs")
	if _, err := os.Stat(cvHtmlScript); os.IsNotExist(err) {
		cvHtmlScript = filepath.Join(contextDir, "generate-cv-html.mjs")
	}
	cvHtmlScript, _ = filepath.Abs(cvHtmlScript)
	cmdHtml := exec.CommandContext(ctx, "node", cvHtmlScript, jobURL, company, role, reportPath)
	cmdHtml.Dir = contextDir
	cmdHtml.Stderr = os.Stderr
	cmdHtml.Stdout = os.Stderr

	// Propagate LLM base URL and set model
	var baseUrl, apiKey, llmModel string

	if engine == llm.EngineOpenAI {
		baseUrl = "https://api.openai.com/v1"
		apiKey = os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			apiKey = cfg.LLMAPIKey
		}
		llmModel = cfg.FreeLLMAPIModel
		if llmModel == "" || llmModel == "auto" {
			llmModel = "gpt-4o-mini"
		}
	} else {
		baseUrl = cfg.FreeLLMAPIBaseURL
		apiKey = cfg.LLMAPIKey
		llmModel = cfg.FreeLLMAPIModel
		if llmModel == "" {
			llmModel = "auto"
		}
	}

	cmdHtml.Env = append(os.Environ(),
		"LLM_BASE_URL="+baseUrl,
		"LLM_API_KEY="+apiKey,
		"LLM_MODEL="+llmModel,
	)

	if err := cmdHtml.Run(); err != nil {
		return nil, fmt.Errorf("failed running scripts/generate-cv-html.mjs: %w", err)
	}

	// Verify HTML is generated
	htmlPath := filepath.Join(outputDir, fmt.Sprintf("test-cv-%s.html", company))
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("expected HTML output not found at %s", htmlPath)
	}
	defer os.Remove(htmlPath) // Cleanup raw html

	// Read and validate HTML for unresolved placeholders
	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading generated CV HTML: %w", err)
	}
	if err := ValidateCVHTML(string(htmlBytes)); err != nil {
		return nil, err
	}

	// Step 2: Render HTML to PDF via generate-pdf.mjs (Playwright)
	pdfRelPath := filepath.Join("output", fmt.Sprintf("cv-%s.pdf", company))
	pdfAbsPath := filepath.Join(contextDir, pdfRelPath)

	// FRAGILE: Using Playwright via node to render PDFs adds heavy system requirements and runtime overhead.
	// TODO: Transition to a Go-native PDF generation library or a specialized rendering microservice.
	// Resolve script path: prefer root of contextDir, fall back to scripts/ subdirectory.
	pdfScript := filepath.Join(contextDir, "generate-pdf.mjs")
	if _, err := os.Stat(pdfScript); os.IsNotExist(err) {
		pdfScript = filepath.Join(contextDir, "scripts", "generate-pdf.mjs")
	}
	pdfScript, _ = filepath.Abs(pdfScript)
	cmdPdf := exec.CommandContext(ctx, "node", pdfScript, filepath.Join("output", fmt.Sprintf("test-cv-%s.html", company)), pdfRelPath)
	cmdPdf.Dir = contextDir
	cmdPdf.Stderr = os.Stderr
	cmdPdf.Stdout = os.Stderr

	if err := cmdPdf.Run(); err != nil {
		return nil, fmt.Errorf("failed running generate-pdf.mjs: %w", err)
	}
	defer os.Remove(pdfAbsPath) // Cleanup generated PDF after we read it

	// Read generated PDF bytes
	pdfBytes, err := os.ReadFile(pdfAbsPath)
	if err != nil {
		return nil, fmt.Errorf("failed reading generated CV PDF: %w", err)
	}

	return pdfBytes, nil
}
