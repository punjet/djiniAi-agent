package covergen

import (
	"context"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/logger"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func preprocessTemplate(htmlContent string) string {
	return placeholderPattern.ReplaceAllString(htmlContent, `{{.$1}}`)
}

func getStringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func sanitizeName(name string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	sanitized := reg.ReplaceAllString(name, "")
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	return strings.ToLower(sanitized)
}

// GenerateCoverLetter calls the LLM to draft a cover letter and then renders
// it to PDF using Go html/template and chromedp.
// Returns the PDF bytes, the Djinni message text, and any error.
func GenerateCoverLetter(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir string, company string, role string, jdText string) ([]byte, string, error) {
	logger.Log.Info("Starting cover letter generation for", "company", company, "role", role)

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
	provider, err := llm.NewProvider(cfg, engine, "resume")
	if err != nil {
		return nil, "", err
	}

	lang := DetectJDLanguage(jdText)

	var systemPrompt string
	if lang == "English" {
		systemPrompt = `You are an expert technical resume writer. Write a highly tailored cover letter and a short Djinni message hook for a candidate applying to a job.

 CRITICAL LANGUAGE RULE: You MUST write ALL text content STRICTLY in English.
This means:
- Translate ALL content — greeting, opening, profile intro, achievements, problems section, closing, and Djinni message — into English.
- The only exception is proper nouns (company names, product names, tool names like "n8n", "RAG", "OpenAI").

Instructions:
Draft a cover letter and short message following these guidelines (inspired by Santiago's career-ops):
1. Greeting: tailored to the company or hiring manager.
2. Opening: clear statement of application, role title, and immediate hook showing you understand their domain.
3. Profile Intro: 2-3 sentences matching the candidate's core narrative to the role.
4. Achievements: 2-3 achievements tailored to the role, with a 'lead' (what candidate did) and 'impact' (quantified results).
5. Problems Section: explain how candidate's superpowers solve their specific challenges.
6. Closing: selective, direct, confident.
7. Djinni Message: A short, concise hook (3-4 sentences) for the initial message. Do not make it generic. Highlight the candidate's core value match.
8. NO MARKDOWN: Do not use ANY markdown formatting (like **, *, #) anywhere in your response. The output must be pure plain text.

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
	} else {
		systemPrompt = `Ви є досвідченим автором технічних резюме. Напишіть індивідуальний супровідний лист та короткий гачок для повідомлення на Djinni для кандидата, який подає заявку на вакансію.

 КРИТИЧНЕ ПРАВИЛО МОВИ: Ви ПОВИННІ писати ВЕСЬ текст ВИКЛЮЧНО українською мовою.
Це означає:
- Перекладіть ВЕСЬ вміст — привітання, вступ, опис профілю, досягнення, розділ про проблеми, закінчення та повідомлення на Djinni — українською мовою.
- Єдиним винятком є власні назви (назви компаній, назви продуктів, назви інструментів, наприклад "n8n", "RAG", "OpenAI").

Інструкції:
Складіть супровідний лист та коротке повідомлення, дотримуючись наступних рекомендацій:
1. Привітання: адаптовано до компанії або менеджера з найму.
2. Вступ: чітка заява про подачу заявки, назву посади та негайний гачок, що показує розуміння їхньої сфери діяльності.
3. Опис профілю: 2-3 речення, що пов'язують основну історію кандидата з цією роллю.
4. Досягнення: 2-3 досягнення, адаптовані до ролі, з описом "що зробив кандидат" (lead) та "вплив" (impact, кількісні результати).
5. Розділ про проблеми: поясніть, як суперсили кандидата вирішують їхні конкретні виклики.
6. Закінчення: вибіркове, пряме, впевнене.
7. Повідомлення на Djinni: короткий, лаконічний гачок (3-4 речення) для першого повідомлення. Не робіть його загальним. Виділіть відповідність ключових переваг кандидата вимогам вакансії.
8. БЕЗ MARKDOWN: Не використовуйте ЖОДНОГО форматування markdown (наприклад, **, *, #) ніде у вашій відповіді. Результат має бути чистим простим текстом.

Ви ПОВИННІ відповісти одним об'єктом JSON (без обгорток markdown, таких як ` + "`" + `json, та без коментарів), який точно відповідає цій схемі:
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
	}

	var userPrompt string
	if lang == "English" {
		userPrompt = fmt.Sprintf(`Candidate Context:
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
	} else {
		userPrompt = fmt.Sprintf(`Контекст кандидата:
Ім'я: %s
Email: %s
Телефон: %s
Локація: %s
Linkedin: %s
Github: %s
Вміст резюме:
%s

Деталі вакансії:
Компанія: %s
Роль: %s
Вміст вакансії:
%s`, prof.Candidate.FullName, prof.Candidate.Email, prof.Candidate.Phone, prof.Candidate.Location, prof.Candidate.Linkedin, prof.Candidate.Github, string(cvData), company, role, jdText)
	}

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

	// 5. Render cover letter template to HTML and convert to PDF
	templatePath := filepath.Join(contextDir, "templates", "cover-letter-template.html")
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read cover letter template: %w", err)
	}

	var achievementsBlock strings.Builder
	if raw, ok := genLetter.Letter["achievements"]; ok {
		if achievements, ok := raw.([]interface{}); ok {
			achievementsBlock.WriteString(`<ul class="achievements">`)
			for _, a := range achievements {
				if m, ok := a.(map[string]interface{}); ok {
					lead, _ := m["lead"].(string)
					impact, _ := m["impact"].(string)
					achievementsBlock.WriteString(fmt.Sprintf("<li><strong>%s</strong> — %s</li>", lead, impact))
				}
			}
			achievementsBlock.WriteString("</ul>")
		}
	}

	problemsBlock := ""
	if problems := getStringField(genLetter.Letter, "problems_section"); problems != "" {
		problemsBlock = fmt.Sprintf("<p>%s</p>", problems)
	}

	closingBlock := ""
	if closing := getStringField(genLetter.Letter, "closing"); closing != "" {
		closingBlock = fmt.Sprintf("<p>%s</p>", closing)
	}

	greeting := getStringField(genLetter.Letter, "greeting")
	greetingBlock := ""
	if greeting != "" {
		greetingBlock = fmt.Sprintf(`<p class="greeting">%s</p>`, greeting)
	}

	today := time.Now().Format("2006-01-02")

	tmplData := map[string]string{
		"NAME":                   sanitizeName(prof.Candidate.FullName),
		"CONTACT_LINE":           fmt.Sprintf("%s | %s", prof.Candidate.Email, prof.Candidate.Phone),
		"CREDENTIALS_BLOCK":      "",
		"ROLE_TITLE":             role,
		"DATELINE":               today,
		"GREETING_BLOCK":         greetingBlock,
		"OPENING":                getStringField(genLetter.Letter, "opening"),
		"PROFILE_INTRO":          getStringField(genLetter.Letter, "profile_intro"),
		"ACHIEVEMENTS_BLOCK":     achievementsBlock.String(),
		"PROBLEMS_BLOCK":         problemsBlock,
		"CLOSING_BLOCK":          closingBlock,
		"LANGUAGE_CLOSING_BLOCK": "",
		"FOOTNOTES_BLOCK":        "",
	}

	tmpl, err := template.New("cover-letter").Parse(preprocessTemplate(string(tmplBytes)))
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse cover letter template: %w", err)
	}

	var htmlBuf strings.Builder
	if err := tmpl.Execute(&htmlBuf, tmplData); err != nil {
		return nil, "", fmt.Errorf("failed to execute cover letter template: %w", err)
	}

	pdfBytes, err := renderHTMLToPDF(ctx, htmlBuf.String())
	if err != nil {
		return nil, "", fmt.Errorf("failed to render cover letter PDF: %w", err)
	}

	pdfOutFilename := fmt.Sprintf("%s-%s-cover.pdf", strings.ToLower(company), strings.ToLower(role))
	pdfOutFilename = regexp.MustCompile(`[^a-z0-9.-]+`).ReplaceAllString(pdfOutFilename, "-")
	pdfOutPath := filepath.Join(contextDir, "output", pdfOutFilename)

	if err := os.MkdirAll(filepath.Dir(pdfOutPath), 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.WriteFile(pdfOutPath, pdfBytes, 0644); err != nil {
		return nil, "", fmt.Errorf("failed to write PDF: %w", err)
	}

	return pdfBytes, genLetter.DjinniMessage, nil
}

// AnswerQuizQuestions calls the LLM to answer recruiter quiz questions based on the candidate's CV and profile.
// It loads cv.md and config/profile.yml from contextDir, sends the questions to the LLM,
// and populates the Answer field on each question from the JSON response.
