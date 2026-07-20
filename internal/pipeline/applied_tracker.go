// Package pipeline provides core processing functions for the career-ops toolchain.
//
// This file implements a simple tracker for jobs that have been applied to.
// It stores a map of job IDs to the timestamp when the application was made.
// The data is persisted in `data/applied_jobs.json` as a JSON object where the
// values are RFC3339 timestamps. The implementation ensures atomic writes and
// concurrency safety via a package‑level mutex.
package pipeline

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

// fileMutex protects concurrent access to the applied jobs file.
var fileMutex sync.Mutex

// appliedJobsPath is the location of the JSON file relative to the project root.
var appliedJobsPath = filepath.Join("data", "applied_jobs.json")

// LoadAppliedJobs reads the applied_jobs.json file and returns a map of job ID to
// the timestamp when the job was applied. If the file does not exist, an empty
// map is returned. The timestamps are parsed using time.RFC3339.
func LoadAppliedJobs() (map[string]time.Time, error) {
    // Read the file; if missing return empty map.
    data, err := os.ReadFile(appliedJobsPath)
    if err != nil {
        if os.IsNotExist(err) {
            return map[string]time.Time{}, nil
        }
        return nil, fmt.Errorf("failed to read applied jobs file: %w", err)
    }

    // Unmarshal into a map of strings.
    var raw map[string]string
    if err := json.Unmarshal(data, &raw); err != nil {
        return nil, fmt.Errorf("failed to unmarshal applied jobs JSON: %w", err)
    }

    // Convert to map[string]time.Time.
    result := make(map[string]time.Time, len(raw))
    for id, ts := range raw {
        t, err := time.Parse(time.RFC3339, ts)
        if err != nil {
            // Skip malformed timestamps but continue processing.
            continue
        }
        result[id] = t
    }
    return result, nil
}

// SaveAppliedJob records a new applied job ID with the current UTC timestamp.
// It loads the existing map, updates it, and writes the JSON back atomically.
func SaveAppliedJob(jobID string) error {
    fileMutex.Lock()
    defer fileMutex.Unlock()

    // Load existing data.
    jobs, err := LoadAppliedJobs()
    if err != nil {
        return err
    }
    // Update the map with the current timestamp.
    jobs[jobID] = time.Now().UTC()

    // Convert map to the string representation for JSON storage.
    raw := make(map[string]string, len(jobs))
    for id, t := range jobs {
        raw[id] = t.Format(time.RFC3339)
    }
    jsonData, err := json.MarshalIndent(raw, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal applied jobs: %w", err)
    }

    // Ensure the directory exists.
    dir := filepath.Dir(appliedJobsPath)
    if err := os.MkdirAll(dir, 0o755); err != nil {
        return fmt.Errorf("failed to create directory for applied jobs: %w", err)
    }

    // Write to a temporary file first.
    tmpPath := appliedJobsPath + ".tmp"
    if err := os.WriteFile(tmpPath, jsonData, 0o644); err != nil {
        return fmt.Errorf("failed to write temporary applied jobs file: %w", err)
    }
    // Atomically replace the target file.
    if err := os.Rename(tmpPath, appliedJobsPath); err != nil {
        return fmt.Errorf("failed to rename temporary applied jobs file: %w", err)
    }
    return nil
}
