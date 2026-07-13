package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"djinni-bot-go/internal/api"
	"djinni-bot-go/internal/client"
	"djinni-bot-go/internal/extractor"

	"gopkg.in/yaml.v3"
)

type PortalsConfig struct {
	TitleFilter struct {
		Positive []string `yaml:"positive"`
		Negative []string `yaml:"negative"`
	} `yaml:"title_filter"`
	SearchQueries []map[string]string `yaml:"search_queries"`
}

// LoadPortalsConfig loads the portals configuration from portals.yml.
func LoadPortalsConfig(contextDir string) (*PortalsConfig, error) {
	path := filepath.Join(contextDir, "portals.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg PortalsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse portals.yml: %w", err)
	}
	return &cfg, nil
}

// LoadTitleFilter returns a function that checks if a job title matches config criteria.
func LoadTitleFilter(contextDir string) (func(string) bool, error) {
	cfg, err := LoadPortalsConfig(contextDir)
	if err != nil {
		// If not found, use a fallback filter
		return func(title string) bool {
			lower := strings.ToLower(title)
			return strings.Contains(lower, "ai") || strings.Contains(lower, "llm") || strings.Contains(lower, "agent")
		}, nil
	}

	pos := make([]string, len(cfg.TitleFilter.Positive))
	for i, k := range cfg.TitleFilter.Positive {
		pos[i] = strings.ToLower(k)
	}
	neg := make([]string, len(cfg.TitleFilter.Negative))
	for i, k := range cfg.TitleFilter.Negative {
		neg[i] = strings.ToLower(k)
	}

	return func(title string) bool {
		lower := strings.ToLower(title)
		hasPos := len(pos) == 0
		for _, k := range pos {
			if strings.Contains(lower, k) {
				hasPos = true
				break
			}
		}
		hasNeg := false
		for _, k := range neg {
			if strings.Contains(lower, k) {
				hasNeg = true
				break
			}
		}
		return hasPos && !hasNeg
	}, nil
}

// ScanDjinni fetches jobs from candidate's dashboard and keyword searches,
// filters them by title and deduplicates them using Dedup.
func ScanDjinni(contextDir string, dc *client.DjinniClient, dedup *Dedup) ([]extractor.JobSummary, error) {
	filter, err := LoadTitleFilter(contextDir)
	if err != nil {
		return nil, err
	}

	var allJobs []extractor.JobSummary
	seen := make(map[string]bool)

	addJobs := func(jobs []extractor.JobSummary) {
		for _, j := range jobs {
			if seen[j.Slug] {
				continue
			}
			seen[j.Slug] = true

			// Apply title filter
			if !filter(j.Title) {
				continue
			}

			// Apply deduplication (check seen URLs)
			if !dedup.IsNew(j.URL, "", "") {
				continue
			}

			allJobs = append(allJobs, j)
		}
	}

	// 1. Fetch dashboard jobs (recommended for candidate)
	fmt.Println("Scraping Djinni candidate dashboard (up to 5 pages)...")
	for page := 1; page <= 5; page++ {
		dashJobs, err := api.GetDashboardJobs(dc, page)
		if err == nil {
			if len(dashJobs) == 0 {
				break // no more jobs on this page, stop paginating
			}
			addJobs(dashJobs)
			fmt.Printf("Dashboard page %d: found %d relevant new jobs\n", page, len(dashJobs))
		} else {
			fmt.Printf("Warning: failed to fetch dashboard jobs on page %d: %v\n", page, err)
			break
		}
	}

	// 2. Run searches for key titles
	var searchQueries []map[string]string
	cfg, err := LoadPortalsConfig(contextDir)
	if err == nil && len(cfg.SearchQueries) > 0 {
		searchQueries = cfg.SearchQueries
	} else {
		// FRAGILE: Hardcoded default search queries were previously inlined here.
		// TODO: Move all search definitions completely to the YAML configuration and remove fallback if possible.
		searchQueries = []map[string]string{
			{"title": "AI"},
			{"title": "LLM"},
			{"title": "Agent"},
			{"title": "Automation"},
			{"title": "Automation Expert"},
			{"title": "AI Integrator"},
			{"title": "AI Consultant"},
			{"title": "No-Code"},
			{"title": "Low-Code"},
			{"title": "Make.com"},
			{"title": "n8n"},
			{"title": "Machine Learning"},
			{"title": "Data"},
		}
	}

	for _, query := range searchQueries {
		qName := ""
		for k, v := range query {
			qName = fmt.Sprintf("%s=%s", k, v)
		}
		fmt.Printf("Searching Djinni for: %s...\n", qName)
		jobs, err := api.SearchJobs(dc, query)
		if err == nil {
			addJobs(jobs)
		} else {
			fmt.Printf("Warning: search failed for %s: %v\n", qName, err)
		}
	}

	return allJobs, nil
}
