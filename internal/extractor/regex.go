package extractor

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Job represents a summary of a job posting.
type Job struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type JobSummary = Job

// QuizQuestion represents a single question in a recruiter quiz.
type QuizQuestion struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Answer string `json:"answer,omitempty"`
}

// JobDetails represents the full details of a job posting.
type JobDetails struct {
	Title         string         `json:"title"`
	Company       string         `json:"company"`
	Description   string         `json:"description"`
	Requirements  string         `json:"requirements"`
	QuizID        string         `json:"quiz_id"`
	QuizQuestions []QuizQuestion `json:"quiz_questions,omitempty"`
}

var (
	htmlTagRegex    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRegex = regexp.MustCompile(`\s+`)
	companyCleanRx  = regexp.MustCompile(`[·\.]`)

	reqHeaders = []*regexp.Regexp{
		regexp.MustCompile(`(?i)requirements?:?([\s\S]+)`),
		regexp.MustCompile(`(?i)вимоги:?([\s\S]+)`),
		regexp.MustCompile(`(?i)наші очікування:?([\s\S]+)`),
		regexp.MustCompile(`(?i)очікуємо від вас:?([\s\S]+)`),
	}
)

// ExtractCSRF extracts the CSRF token from the HTML body using goquery DOM parsing.
func ExtractCSRF(htmlContent string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var token string
	doc.Find("input[name='csrfmiddlewaretoken']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("value"); exists && val != "" {
			token = val
		}
	})

	if token != "" {
		return token, nil
	}
	return "", errors.New("csrf token not found")
}

// ExtractJobs extracts job postings from a job search/list HTML page using goquery.
func ExtractJobs(htmlContent string) ([]Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var jobs []Job
	seenSlugs := make(map[string]bool)

	doc.Find("a[href^='/jobs/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		parts := strings.Split(strings.Trim(href, "/"), "/")
		if len(parts) < 2 || parts[0] != "jobs" {
			return
		}

		slugPart := parts[1]
		dashIndex := strings.Index(slugPart, "-")
		if dashIndex <= 0 {
			return
		}

		id := slugPart[:dashIndex]
		slugSuffix := slugPart[dashIndex+1:]
		if slugSuffix == "" {
			return
		}

		title := cleanTitle(s.Text())

		// Exclude search links that are short or match login/signup strings
		if len(title) < 3 ||
			strings.Contains(title, "Увійти") ||
			strings.Contains(title, "Зареєструватись") ||
			strings.Contains(title, "Log in") ||
			strings.Contains(title, "Sign up") ||
			seenSlugs[slugPart] {
			return
		}

		seenSlugs[slugPart] = true
		jobs = append(jobs, Job{
			ID:    id,
			Slug:  slugPart,
			Title: title,
			URL:   fmt.Sprintf("https://djinni.co/jobs/%s/", slugPart),
		})
	})

	return jobs, nil
}

// ExtractDashboardJobs extracts job postings from the personal dashboard HTML page using goquery.
func ExtractDashboardJobs(htmlContent string) ([]Job, error) {
	return ExtractDashboardJobsV2(htmlContent)
}

// jobPostingLD holds structured application/ld+json data.
type jobPostingLD struct {
	Type               string `json:"@type"`
	Title              string `json:"title"`
	HiringOrganization struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
	Description string `json:"description"`
}

// ExtractJobDetails extracts detailed information from a job posting HTML page using goquery DOM parsing.
func ExtractJobDetails(htmlContent string) (*JobDetails, error) {
	return ExtractJobDetailsV2(htmlContent)
}

func cleanTitle(title string) string {
	title = html.UnescapeString(title)
	title = htmlTagRegex.ReplaceAllString(title, "")
	title = whitespaceRegex.ReplaceAllString(title, " ")
	return strings.TrimSpace(title)
}

func cleanDescription(str string) string {
	if str == "" {
		return ""
	}
	str = regexp.MustCompile(`(?i)</p>`).ReplaceAllString(str, "\n\n")
	str = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(str, "\n")
	str = regexp.MustCompile(`(?i)</li>`).ReplaceAllString(str, "\n")
	str = htmlTagRegex.ReplaceAllString(str, "")
	str = html.UnescapeString(str)
	str = strings.ReplaceAll(str, "\u00a0", " ")

	lines := strings.Split(str, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
