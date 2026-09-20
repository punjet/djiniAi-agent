package llm

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

var trailingCommaRegex = regexp.MustCompile(`,(\s*[\}\]])`)

// CleanJSON cleans and repairs an LLM response string containing JSON.
// It strips markdown code blocks (e.g. ```json ... ```), isolates the outermost JSON
// object `{...}` or array `[...]`, removes trailing commas before closing braces/brackets,
// and ensures valid JSON bytes can be unmarshaled.
func CleanJSON(s string) string {
	s = strings.TrimSpace(s)

	// 1. Strip markdown code block fences if present
	if strings.HasPrefix(s, "```") {
		// Remove leading ``` or ```json or similar
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		} else {
			s = strings.TrimPrefix(s, "```")
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}

	// Also handle embedded markdown block if fences weren't at the very start
	if idx := strings.Index(s, "```json"); idx != -1 {
		rest := s[idx+7:]
		if endIdx := strings.Index(rest, "```"); endIdx != -1 {
			s = rest[:endIdx]
		}
		s = strings.TrimSpace(s)
	} else if idx := strings.Index(s, "```"); idx != -1 {
		rest := s[idx+3:]
		if endIdx := strings.Index(rest, "```"); endIdx != -1 {
			s = rest[:endIdx]
		}
		s = strings.TrimSpace(s)
	}

	// 2. Find outermost JSON object or array bounds
	firstObj := strings.Index(s, "{")
	firstArr := strings.Index(s, "[")

	start := -1
	end := -1

	if firstObj != -1 && (firstArr == -1 || firstObj < firstArr) {
		start = firstObj
		end = strings.LastIndex(s, "}")
	} else if firstArr != -1 {
		start = firstArr
		end = strings.LastIndex(s, "]")
	}

	if start != -1 && end != -1 && end > start {
		s = s[start : end+1]
	}

	// 3. Remove trailing commas before } or ]
	s = removeTrailingCommas(s)

	return strings.TrimSpace(s)
}

// removeTrailingCommas strips trailing commas before } or ] while ignoring commas inside string literals.
func removeTrailingCommas(input string) string {
	// Simple regex replacement for typical formatted/unformatted JSON
	// We run it repeatedly in case of nested trailing commas like `, }, ]`
	res := input
	for {
		cleaned := trailingCommaRegex.ReplaceAllString(res, "$1")
		if cleaned == res {
			break
		}
		res = cleaned
	}

	// Verify if json.Valid passes. If it passes, return immediately.
	if json.Valid([]byte(res)) {
		return res
	}

	// If regex replacement didn't fix invalid JSON due to string literal false matches or complex structure,
	// run state machine tokenizer pass.
	return sanitizeTrailingCommasStateful(input)
}

func sanitizeTrailingCommasStateful(input string) string {
	var buf bytes.Buffer
	inString := false
	escaped := false

	runes := []rune(input)
	n := len(runes)

	for i := 0; i < n; i++ {
		ch := runes[i]

		if inString {
			buf.WriteRune(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}

		if ch == '"' {
			inString = true
			buf.WriteRune(ch)
			continue
		}

		if ch == ',' {
			// Look ahead for next non-whitespace character
			j := i + 1
			for j < n && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
				j++
			}
			if j < n && (runes[j] == '}' || runes[j] == ']') {
				// Skip this comma!
				continue
			}
		}

		buf.WriteRune(ch)
	}

	return buf.String()
}
