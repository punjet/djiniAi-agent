package covergen

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/extractor"
	"djinni-bot-go/internal/llm"
	"djinni-bot-go/internal/logger"

	"github.com/abadojack/whatlanggo"
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

type CVContent struct {
	SummaryText        string `json:"summary_text"`
	CompetenciesHTML   string `json:"competencies_html"`
	ExperienceHTML     string `json:"experience_html"`
	ProjectsHTML       string `json:"projects_html"`
	EducationHTML      string `json:"education_html"`
	CertificationsHTML string `json:"certifications_html"`
	SkillsHTML         string `json:"skills_html"`
}

var placeholderPattern = regexp.MustCompile(`\{\{([A-Z][A-Z_]+)\}\}`)

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

⚠️ CRITICAL LANGUAGE RULE: You MUST write ALL text content STRICTLY in English.
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

⚠️ КРИТИЧНЕ ПРАВИЛО МОВИ: Ви ПОВИННІ писати ВЕСЬ текст ВИКЛЮЧНО українською мовою.
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
	provider, err := llm.NewProvider(cfg, engine, "resume")
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

func ValidateCVHTML(htmlContent string) error {
	if strings.Contains(htmlContent, "{{") || strings.Contains(htmlContent, "}}") {
		return fmt.Errorf("generated CV contains unresolved template placeholders (found '{{' or '}}')")
	}
	return nil
}

func GenerateCustomCV(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir, jobURL, company, role, reportPath string) ([]byte, error) {
	profilePath := filepath.Join(contextDir, "config", "profile.yml")
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile.yml: %w", err)
	}

	var prof Profile
	if err := yaml.Unmarshal(profileData, &prof); err != nil {
		return nil, fmt.Errorf("failed to parse profile.yml: %w", err)
	}

	cvPath := filepath.Join(contextDir, "cv.md")
	cvData, err := os.ReadFile(cvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cv.md: %w", err)
	}

	var reportText string
	if reportPath != "" {
		reportBytes, err := os.ReadFile(reportPath)
		if err == nil {
			reportText = string(reportBytes)
		}
	}

	provider, err := llm.NewProvider(cfg, engine, "resume")
	if err != nil {
		return nil, err
	}

	// Detect JD language — same mechanism used in GenerateCoverLetter
	var jobText string
	if reportText != "" {
		jobText = reportText
	} else {
		jobText = fmt.Sprintf("%s %s", company, role)
	}
	lang := DetectJDLanguage(jobText)
	logger.Log.Info("CV language detected", "language", lang)

	var systemPrompt string
	var userPrompt string
	if lang == "English" {
		systemPrompt = `You are an expert CV writer. Generate a tailored CV in JSON for the candidate based on their profile and the target job.

⚠️ CRITICAL LANGUAGE RULE: You MUST write ALL text content STRICTLY in English.
This means:
- Translate ALL experience descriptions, project descriptions, education entries from their original language into English.
- Write the professional summary in English.
- Write competency tags in English.
- Write skill items in English.
- The only exception is proper nouns (company names, product names, tool names).

You MUST respond with a single JSON object (no markdown wrappers, no comments) matching this schema exactly:
{
  "summary_text": "Comprehensive professional summary tailored to the role (4-6 sentences). Highlight the candidate's unique value proposition, core expertise, and fit for this specific role.",
  "competencies_html": "HTML string of ALL competency tags from the CV, each as <span class=\"competency-tag\">Skill</span>. Include all relevant skills, do not omit any.",
  "experience_html": "Full detailed HTML for ALL work experience entries. Each job as a .job div with .job-header containing .job-company and .job-period, .job-role, and a ul with li items. Include ALL bullet points from the CV for each role — do NOT summarize or truncate.",
  "projects_html": "Full detailed HTML for ALL projects. Each project as a .project div with .project-title and .project-desc. Translate and include ALL project details, technical stack, and outcomes from the CV.",
  "education_html": "Full HTML for ALL education entries - each item as .edu-item with .edu-header containing .edu-title, .edu-org, .edu-year, and optional .edu-desc",
  "certifications_html": "Full HTML for ALL certifications and awards - each as .cert-item with .cert-title, .cert-org, .cert-year",
  "skills_html": "HTML string of ALL skill categories, each as <span class=\"skill-item\"><span class=\"skill-category\">Category:</span> skill list</span>"
}

IMPORTANT: Be detailed and comprehensive. Include ALL information from the candidate's CV. Do not summarize or shorten. The output should be a full, rich CV — not a skeleton.
All HTML must be clean, valid HTML fragments. Use <strong> for emphasis. Never include markdown formatting.`

		userPrompt = fmt.Sprintf(`Candidate Profile:
Name: %s
Email: %s
Location: %s
LinkedIn: %s
GitHub: %s

CV/Resume:
%s

Target Job:
Company: %s
Role: %s
Job URL: %s

Job Report:
%s

Remember: translate ALL content to English. Generate a tailored CV JSON for this candidate targeting the above role. Return JSON only.`,
			prof.Candidate.FullName, prof.Candidate.Email, prof.Candidate.Location,
			prof.Candidate.Linkedin, prof.Candidate.Github,
			string(cvData), company, role, jobURL, reportText)
	} else {
		systemPrompt = `Ви є досвідченим автором резюме. Згенеруйте адаптоване резюме в форматі JSON для кандидата на основі його профілю та цільової вакансії.

⚠️ КРИТИЧНЕ ПРАВИЛО МОВИ: Ви ПОВИННІ писати ВЕСЬ текстовий вміст ВИКЛЮЧНО українською мовою.
Це означає:
- Перекладіть ВСІ описи досвіду роботи, описи проєктів, записи про освіту з їхньої оригінальної мови на українську.
- Напишіть професійне резюме (summary) українською мовою.
- Напишіть теги компетенцій українською мовою.
- Напишіть елементи навичок українською мовою.
- Єдиним винятком є власні назви (назви компаній, назви продуктів, назви інструментів).

Ви ПОВИННІ відповісти одним об'єктом JSON (без обгорток markdown, без коментарів), який точно відповідає цій схемі:
{
  "summary_text": "Комплексне професійне резюме, адаптоване до ролі (4-6 речень). Виділіть унікальну ціннісну пропозицію кандидата, основний досвід та відповідність цій конкретній ролі.",
  "competencies_html": "Рядок HTML ВСІХ тегів компетенцій з резюме, кожен як <span class=\"competency-tag\">Навичка</span>. Включіть усі відповідні навички, нічого не опускайте.",
  "experience_html": "Повний детальний HTML для ВСІХ записів досвіду роботи. Кожна робота як div .job з .job-header, що містить .job-company та .job-period, .job-role, та ul з li елементами. Включіть ВСІ пункти списку (bullet points) з резюме для кожної ролі — НЕ узагальнюйте і НЕ скорочуйте.",
  "projects_html": "Повний детальний HTML для ВСІХ проєктів. Кожен проєкт як div .project з .project-title та .project-desc. Перекладіть та включіть ВСІ деталі проєкту, технічний стек та результати з резюме.",
  "education_html": "Повний HTML для ВСІХ записів про освіту - кожен елемент як .edu-item з .edu-header, що містить .edu-title, .edu-org, .edu-year, та необов'язковий .edu-desc",
  "certifications_html": "Повний HTML для ВСІХ сертифікатів та нагород - кожен як .cert-item з .cert-title, .cert-org, .cert-year",
  "skills_html": "Рядок HTML ВСІХ категорій навичок, кожен як <span class=\"skill-item\"><span class=\"skill-category\">Категорія:</span> список навичок</span>"
}

ВАЖЛИВО: Будьте детальними та вичерпними. Включіть ВСЮ інформацію з резюме кандидата. Не узагальнюйте і не скорочуйте. Результат має бути повним, насиченим резюме — а не скелетом.
Увесь HTML має бути чистими, правильними фрагментами HTML. Використовуйте <strong> для виділення. Ніколи не використовуйте форматування markdown.`

		userPrompt = fmt.Sprintf(`Профіль кандидата:
Ім'я: %s
Email: %s
Локація: %s
LinkedIn: %s
GitHub: %s

Резюме/CV:
%s

Цільова вакансія:
Компанія: %s
Role: %s
Job URL: %s

Job Report:
%s

Згенеруй адаптоване CV у форматі JSON для цього кандидата. Повертай тільки JSON.`,
			prof.Candidate.FullName, prof.Candidate.Email, prof.Candidate.Location,
			prof.Candidate.Linkedin, prof.Candidate.Github,
			string(cvData), company, role, jobURL, reportText)
	}

	response, err := provider.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM CV generation failed: %w", err)
	}

	cleanJSON := response
	if idx := strings.Index(cleanJSON, "{"); idx != -1 {
		cleanJSON = cleanJSON[idx:]
	}
	if idx := strings.LastIndex(cleanJSON, "}"); idx != -1 {
		cleanJSON = cleanJSON[:idx+1]
	}

	var content CVContent
	if err := json.Unmarshal([]byte(cleanJSON), &content); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response JSON: %w (raw: %q)", err, response)
	}

	templatePath := filepath.Join(contextDir, "templates", "cv-template.html")
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CV template: %w", err)
	}

	linkedinDisplay := strings.TrimPrefix(prof.Candidate.Linkedin, "https://")
	linkedinDisplay = strings.TrimPrefix(linkedinDisplay, "http://")
	portfolioDisplay := strings.TrimPrefix(prof.Candidate.Github, "https://")
	portfolioDisplay = strings.TrimPrefix(portfolioDisplay, "http://")

	// Section labels — localized based on detected JD language
	sectionSummary := "Professional Summary"
	sectionCompetencies := "Core Competencies & Technologies"
	sectionExperience := "Professional Experience"
	sectionProjects := "Projects"
	sectionEducation := "Education"
	sectionCertifications := "Certifications & Awards"
	sectionSkills := "Technical Skills"

	if lang == "Ukrainian" {
		sectionSummary = "Професійне резюме"
		sectionCompetencies = "Ключові компетенції та технології"
		sectionExperience = "Досвід роботи"
		sectionProjects = "Проєкти"
		sectionEducation = "Освіта"
		sectionCertifications = "Сертифікати та нагороди"
		sectionSkills = "Технічні навички"
	}

	tmplData := map[string]interface{}{
		"LANG":                   "en",
		"NAME":                   prof.Candidate.FullName,
		"EMAIL":                  prof.Candidate.Email,
		"LINKEDIN_URL":           prof.Candidate.Linkedin,
		"LINKEDIN_DISPLAY":       linkedinDisplay,
		"PORTFOLIO_URL":          prof.Candidate.Github,
		"PORTFOLIO_DISPLAY":      portfolioDisplay,
		"LOCATION":               prof.Candidate.Location,
		"PAGE_WIDTH":             "900px",
		"SECTION_SUMMARY":        sectionSummary,
		"SUMMARY_TEXT":           content.SummaryText,
		"SECTION_COMPETENCIES":   sectionCompetencies,
		"COMPETENCIES":           template.HTML(content.CompetenciesHTML),
		"SECTION_EXPERIENCE":     sectionExperience,
		"EXPERIENCE":             template.HTML(content.ExperienceHTML),
		"SECTION_PROJECTS":       sectionProjects,
		"PROJECTS":               template.HTML(content.ProjectsHTML),
		"SECTION_EDUCATION":      sectionEducation,
		"EDUCATION":              template.HTML(content.EducationHTML),
		"SECTION_CERTIFICATIONS": sectionCertifications,
		"CERTIFICATIONS":         template.HTML(content.CertificationsHTML),
		"SECTION_SKILLS":         sectionSkills,
		"SKILLS":                 template.HTML(content.SkillsHTML),
	}

	tmpl, err := template.New("cv").Parse(preprocessTemplate(string(tmplBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse CV template: %w", err)
	}

	var htmlBuf strings.Builder
	if err := tmpl.Execute(&htmlBuf, tmplData); err != nil {
		return nil, fmt.Errorf("failed to execute CV template: %w", err)
	}

	htmlStr := htmlBuf.String()

	if err := ValidateCVHTML(htmlStr); err != nil {
		return nil, err
	}

	pdfBytes, err := renderHTMLToPDF(ctx, htmlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to render CV PDF: %w", err)
	}

	outputDir := filepath.Join(contextDir, "output")
	_ = os.MkdirAll(outputDir, 0755)

	return pdfBytes, nil
}

func DetectJDLanguage(jdText string) string {
	info := whatlanggo.Detect(jdText)
	if info.Lang == whatlanggo.Eng {
		return "English"
	}
	return "Ukrainian"
}
