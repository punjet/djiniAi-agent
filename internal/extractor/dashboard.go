package extractor

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractDashboardJobsV2 extracts job postings from the personal dashboard HTML page using goquery.
func ExtractDashboardJobsV2(htmlContent string) ([]Job, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var jobs []Job
	seenSlugs := make(map[string]bool)

	// In Djinni, dashboard jobs are usually items in a list. The links usually look like /jobs/ID-SLUG/
	doc.Find("a[href^='/jobs/']").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}

		// Check if it matches /jobs/123-slug/
		parts := strings.Split(strings.Trim(href, "/"), "/")
		if len(parts) < 2 || parts[0] != "jobs" {
			return
		}
		
		slugPart := parts[1]
		
		// Find ID from slug Part
		dashIndex := strings.Index(slugPart, "-")
		if dashIndex <= 0 {
			return
		}
		
		id := slugPart[:dashIndex]

		// The title might be inside a h2 or just the link text
		var title string
		// Try h2 inside the closest wrapper or the link itself
		h2 := s.Find("h2.job-item__position")
		if h2.Length() > 0 {
			title = cleanTitle(h2.Text())
		} else {
			// Find nearest h2 or use text
			title = cleanTitle(s.Text())
		}
		
		if title == "" || len(title) < 3 || strings.Contains(title, "Увійти") || strings.Contains(title, "Зареєструватись") {
			return
		}

		if seenSlugs[slugPart] {
			return
		}
		seenSlugs[slugPart] = true

		jobs = append(jobs, Job{
			ID:    id,
			Slug:  slugPart,
			Title: title,
			URL:   "https://djinni.co/jobs/" + slugPart + "/",
		})
	})

	return jobs, nil
}
