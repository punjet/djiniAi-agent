package extractor

import "strings"

// IsAlreadyApplied checks if the job details HTML page contains the snippet indicating 
// that the candidate has already applied to this job.
func IsAlreadyApplied(html string) bool {
	// Check for standard substring: "You've applied to this job already." or variation
	return strings.Contains(html, "You've applied to this job already.") || 
		strings.Contains(html, "You’ve applied to this job already.")
}
