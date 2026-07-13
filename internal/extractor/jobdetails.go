package extractor

import (
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractJobDetailsV2 extracts detailed information from a job posting HTML page using goquery.
func ExtractJobDetailsV2(htmlContent string) (*JobDetails, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var title, company, description string

	// 1. Try application/ld+json parsing
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		var ld jobPostingLD
		if err := json.Unmarshal([]byte(strings.TrimSpace(s.Text())), &ld); err == nil {
			if ld.Type == "JobPosting" {
				title = ld.Title
				company = ld.HiringOrganization.Name
				description = ld.Description
			}
		}
	})

	// 2. Fallbacks
	if title == "" {
		title = cleanTitle(doc.Find("h1").First().Text())
	}

	if company == "" {
		// Look for company link
		comp := doc.Find("a[href*='/jobs/company-']").First().Text()
		if comp == "" {
			comp = doc.Find(".job-details--company_name, .company_name").First().Text() // fallback generic class
		}
		company = cleanTitle(comp)
	}

	if description == "" {
		description = doc.Find(".job-post__description, .profile-page-section").First().Text()
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

	// 4. Extract quiz questions
	var quizID string
	doc.Find("input[type='hidden'][name='quiz_id']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("value"); exists {
			quizID = val
		}
	})

	var quizQuestions []QuizQuestion
	seenNames := make(map[string]bool)

	doc.Find("input[name^='answer_'], textarea[name^='answer_'], select[name^='answer_']").Each(func(i int, s *goquery.Selection) {
		fieldName, exists := s.Attr("name")
		if !exists || fieldName == "" || seenNames[fieldName] {
			return
		}
		seenNames[fieldName] = true

		questionText := ""

		// 4a. <label for="fieldId"> matching field's id attribute
		if fieldID, hasID := s.Attr("id"); hasID && fieldID != "" {
			if labelText := doc.Find("label[for='" + fieldID + "']").First().Text(); labelText != "" {
				questionText = strings.TrimSpace(labelText)
			}
		}

		// 4b. Ancestor <label> (field is wrapped in a label)
		if questionText == "" {
			if labelText := s.Closest("label").First().Text(); labelText != "" {
				questionText = strings.TrimSpace(labelText)
			}
		}

		// 4c. Sibling <label> in immediate parent
		if questionText == "" {
			if labelText := s.Parent().Find("label").First().Text(); labelText != "" {
				questionText = strings.TrimSpace(labelText)
			}
		}

		// 4d. Sibling <label> in closest ancestor <div>
		if questionText == "" {
			if labelText := s.Closest("div").Find("label").First().Text(); labelText != "" {
				questionText = strings.TrimSpace(labelText)
			}
		}

		questionText = cleanTitle(questionText)

		if questionText != "" {
			quizQuestions = append(quizQuestions, QuizQuestion{
				Name: fieldName,
				Text: questionText,
			})
		}
	})

	if quizQuestions == nil {
		quizQuestions = []QuizQuestion{}
	}

	return &JobDetails{
		Title:         title,
		Company:       strings.TrimSpace(companyCleanRx.ReplaceAllString(company, "")),
		Description:   cleanedDescription,
		Requirements:  requirements,
		QuizID:        quizID,
		QuizQuestions: quizQuestions,
	}, nil
}
