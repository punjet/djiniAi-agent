package pipeline

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Dedup tracks already seen job URLs and company-role combinations.
type Dedup struct {
	seenURLs         map[string]bool
	seenCompanyRoles map[string]bool
}

// cleanURL removes query parameters and fragments.
func cleanURL(u string) string {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		if i := strings.Index(u, "?"); i != -1 {
			return u[:i]
		}
		return u
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// LoadDedup reads scan-history.tsv and applications.md to initialize the seen sets.
func LoadDedup(contextDir string) (*Dedup, error) {
	d := &Dedup{
		seenURLs:         make(map[string]bool),
		seenCompanyRoles: make(map[string]bool),
	}

	historyPath := filepath.Join(contextDir, "data", "scan-history.tsv")
	appsPath := filepath.Join(contextDir, "data", "applications.md")

	// 1. Load from scan-history.tsv
	if file, err := os.Open(historyPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() { // skip header
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Split(line, "\t")
				if len(parts) > 0 && parts[0] != "" {
					d.seenURLs[cleanURL(parts[0])] = true
				}
			}
		}
	}

	// 2. Load from applications.md
	if file, err := os.Open(appsPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		// Match urls in markdown
		urlRx := regexp.MustCompile(`https?://[^\s|)]+`)
		// Match table rows: | # | Date | Company | Role | ...
		rowRx := regexp.MustCompile(`\|[^|]+\|[^|]+\|\s*([^|]+)\s*\|\s*([^|]+)\s*\|`)

		for scanner.Scan() {
			line := scanner.Text()
			// Extract URLs
			for _, match := range urlRx.FindAllString(line, -1) {
				d.seenURLs[cleanURL(match)] = true
			}
			// Extract company & role
			if matches := rowRx.FindStringSubmatch(line); len(matches) > 2 {
				company := normalizeString(matches[1])
				role := normalizeString(matches[2])
				if company != "" && role != "" && company != "company" && company != "empresa" {
					key := company + "::" + role
					d.seenCompanyRoles[key] = true
				}
			}
		}
	}

	return d, nil
}

// IsNew returns true if the URL has not been seen and no duplicate company/role exists.
func (d *Dedup) IsNew(u string, company string, role string) bool {
	if u != "" && d.seenURLs[cleanURL(u)] {
		return false
	}

	normCompany := normalizeString(company)
	normRole := normalizeString(role)
	if normCompany == "" || normRole == "" {
		return true
	}

	// Check exact match
	key := normCompany + "::" + normRole
	if d.seenCompanyRoles[key] {
		return false
	}

	// Fuzzy overlap check (at least 2 overlapping words with len > 3)
	wordsA := getSignifcantWords(normRole)
	if len(wordsA) == 0 {
		return true
	}

	for k := range d.seenCompanyRoles {
		parts := strings.Split(k, "::")
		if len(parts) < 2 {
			continue
		}
		seenCompany := parts[0]
		seenRole := parts[1]

		if seenCompany == normCompany {
			wordsB := getSignifcantWords(seenRole)
			overlap := 0
			for _, wA := range wordsA {
				for _, wB := range wordsB {
					if strings.Contains(wB, wA) || strings.Contains(wA, wB) {
						overlap++
						break
					}
				}
			}
			if overlap >= 2 {
				return false
			}
		}
	}

	return true
}

func normalizeString(s string) string {
	s = strings.ToLower(s)
	// Remove non-alphanumeric except spaces, including unicode letters/numbers
	reg := regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	s = reg.ReplaceAllString(s, "")
	// Normalize spaces
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func getSignifcantWords(s string) []string {
	words := strings.Fields(s)
	var filtered []string
	for _, w := range words {
		// counting runes to correctly handle unicode lengths
		if len([]rune(w)) > 3 {
			filtered = append(filtered, w)
		}
	}
	return filtered
}

// AppendToScanHistory appends a new scanned job to data/scan-history.tsv.
func AppendToScanHistory(contextDir, url, portal, title, company string) {
	historyPath := filepath.Join(contextDir, "data", "scan-history.tsv")

	// Create history file with header if it doesn't exist
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(historyPath), 0o755)
		_ = os.WriteFile(historyPath, []byte("url\tfirst_seen\tportal\ttitle\tcompany\tstatus\n"), 0o644)
	}

	f, err := os.OpenFile(historyPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	today := time.Now().Format("2006-01-02")
	cleanTitle := strings.ReplaceAll(title, "\t", " ")
	cleanCompany := strings.ReplaceAll(company, "\t", " ")

	line := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\tadded\n", url, today, portal, cleanTitle, cleanCompany)
	_, _ = f.WriteString(line)
}
