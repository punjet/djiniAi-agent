package extractor

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// FRAGILE: The extraction logic heavily relies on regular expressions for HTML parsing, which is notoriously brittle.
// TODO: Replace regex-based HTML extraction with a proper DOM parser (e.g., golang.org/x/net/html or goquery).

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
	csrfRegex       = regexp.MustCompile(`(?i)<input[^>]+name="csrfmiddlewaretoken"[^>]+value="([^"]+)"`)
	jobLinkRegex    = regexp.MustCompile(`(?i)<a[^>]+href="/jobs/(\d+)-([^"/?#]+)[^"]*"[^>]*>([\s\S]+?)</a>`)
	htmlTagRegex    = regexp.MustCompile(`<[^>]+>`)
	whitespaceRegex = regexp.MustCompile(`\s+`)
	companyCleanRx  = regexp.MustCompile(`[·\.]`)

	// Job Details Regexes
	jsonLdRegex   = regexp.MustCompile(`(?i)<script[^>]+type="application/ld\+json"[^>]*>([\s\S]+?)</script>`)
	titleRegex    = regexp.MustCompile(`(?i)<h1[^>]*>([\s\S]+?)</h1>`)
	companyDivRx  = regexp.MustCompile(`(?i)<div[^>]+class="[^"]*company_name[^"]*"[^>]*>([\s\S]+?)</div>`)
	companyLinkRx = regexp.MustCompile(`(?i)<a[^>]+href="[^"]*/jobs/company-[^"]*"[^>]*>([\s\S]+?)</a>`)
	descriptionRx = regexp.MustCompile(`(?i)<div[^>]+class="[^"]*job-post__description[^"]*"[^>]*>([\s\S]+?)</div>`)

	reqHeaders = []*regexp.Regexp{
		regexp.MustCompile(`(?i)requirements?:?([\s\S]+)`),
		regexp.MustCompile(`(?i)вимоги:?([\s\S]+)`),
		regexp.MustCompile(`(?i)наші очікування:?([\s\S]+)`),
		regexp.MustCompile(`(?i)очікуємо від вас:?([\s\S]+)`),
	}
)

// ExtractCSRF extracts the CSRF token from the HTML body.
func ExtractCSRF(html string) (string, error) {
	match := csrfRegex.FindStringSubmatch(html)
	if len(match) > 1 {
		return match[1], nil
	}
	return "", errors.New("csrf token not found")
}

// ExtractJobs extracts job postings from a job search/list HTML page.
func ExtractJobs(html string) ([]Job, error) {
	matches := jobLinkRegex.FindAllStringSubmatch(html, -1)
	var jobs []Job
	seenSlugs := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		id := match[1]
		slugSuffix := match[2]
		slug := fmt.Sprintf("%s-%s", id, slugSuffix)
		titleRaw := match[3]

		title := cleanTitle(titleRaw)

		// Exclude search links that are short or match login/signup strings
		if len(title) < 3 ||
			strings.Contains(title, "Увійти") ||
			strings.Contains(title, "Зареєструватись") ||
			strings.Contains(title, "Log in") ||
			strings.Contains(title, "Sign up") ||
			seenSlugs[slug] {
			continue
		}

		seenSlugs[slug] = true
		jobs = append(jobs, Job{
			ID:    id,
			Slug:  slug,
			Title: title,
			URL:   fmt.Sprintf("https://djinni.co/jobs/%s/", slug),
		})
	}

	return jobs, nil
}

// ExtractDashboardJobs extracts job postings from the personal dashboard HTML page.
func ExtractDashboardJobs(html string) ([]Job, error) {
	matches := jobLinkRegex.FindAllStringSubmatch(html, -1)
	var jobs []Job
	seenSlugs := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		id := match[1]
		slugSuffix := match[2]
		slug := fmt.Sprintf("%s-%s", id, slugSuffix)
		innerHTML := match[3]

		// Extract title from <h2 class="job-item__position...">Title</h2>
		titleMatch := regexp.MustCompile(`(?i)<h2[^>]*job-item__position[^>]*>([\s\S]+?)</h2>`).FindStringSubmatch(innerHTML)
		if len(titleMatch) < 2 {
			continue
		}

		title := cleanTitle(titleMatch[1])
		if title == "" || seenSlugs[slug] {
			continue
		}

		seenSlugs[slug] = true
		jobs = append(jobs, Job{
			ID:    id,
			Slug:  slug,
			Title: title,
			URL:   fmt.Sprintf("https://djinni.co/jobs/%s/", slug),
		})
	}

	return jobs, nil
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

// ExtractJobDetails extracts detailed information from a job posting HTML page.
func ExtractJobDetails(html string) (*JobDetails, error) {
	var title, company, description string

	// 1. Try application/ld+json parsing
	jsonLdMatch := jsonLdRegex.FindStringSubmatch(html)
	if len(jsonLdMatch) > 1 {
		var ld jobPostingLD
		if err := json.Unmarshal([]byte(strings.TrimSpace(jsonLdMatch[1])), &ld); err == nil {
			if ld.Type == "JobPosting" {
				title = ld.Title
				company = ld.HiringOrganization.Name
				description = ld.Description
			}
		}
	}

	// 2. Fallbacks
	if title == "" {
		titleMatch := titleRegex.FindStringSubmatch(html)
		if len(titleMatch) > 1 {
			title = cleanTitle(titleMatch[1])
		}
	}

	if company == "" {
		companyDivMatch := companyDivRx.FindStringSubmatch(html)
		if len(companyDivMatch) > 1 {
			company = cleanTitle(companyDivMatch[1])
		} else {
			companyLinkMatch := companyLinkRx.FindStringSubmatch(html)
			if len(companyLinkMatch) > 1 {
				company = cleanTitle(companyLinkMatch[1])
			}
		}
	}

	if description == "" {
		descMatch := descriptionRx.FindStringSubmatch(html)
		if len(descMatch) > 1 {
			description = descMatch[1]
		}
	}

	cleanedDescription := cleanDescription(description)

	// 3. Extract requirements from description
	var requirements string
	for _, rx := range reqHeaders {
		reqMatch := rx.FindStringSubmatch(cleanedDescription)
		if len(reqMatch) > 1 {
			requirements = strings.TrimSpace(reqMatch[1])
			break
		}
	}
	if requirements == "" {
		requirements = cleanedDescription
	}

	return &JobDetails{
		Title:        title,
		Company:      strings.TrimSpace(companyCleanRx.ReplaceAllString(company, "")),
		Description:  cleanedDescription,
		Requirements: requirements,
	}, nil
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
