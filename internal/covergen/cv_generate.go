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
	"strings"

	"gopkg.in/yaml.v3"
)

func GenerateCustomCV(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir, jobURL, company, role, reportPath, jdText string) ([]byte, error) {
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
	if jdText != "" {
		jobText = jdText
	} else if reportText != "" {
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

 CRITICAL LANGUAGE RULE: You MUST write ALL text content STRICTLY in English.
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

 КРИТИЧНЕ ПРАВИЛО МОВИ: Ви ПОВИННІ писати ВЕСЬ текстовий вміст ВИКЛЮЧНО українською мовою.
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

	cleanJSON := llm.CleanJSON(response)

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
