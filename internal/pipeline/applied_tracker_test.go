package pipeline

import (
    "os"
    "testing"
    )

func TestLoadAppliedJobs_Empty(t *testing.T) {
    // Ensure the file does not exist.
    if err := os.Remove(appliedJobsPath); err != nil && !os.IsNotExist(err) {
        t.Fatalf("failed to remove existing applied jobs file: %v", err)
    }
    jobs, err := LoadAppliedJobs()
    if err != nil {
        t.Fatalf("LoadAppliedJobs returned error: %v", err)
    }
    if len(jobs) != 0 {
        t.Fatalf("expected empty map, got %d entries", len(jobs))
    }
}

func TestSaveAndLoadAppliedJob(t *testing.T) {
    // Clean up any existing file before test.
    if err := os.Remove(appliedJobsPath); err != nil && !os.IsNotExist(err) {
        t.Fatalf("failed to remove existing applied jobs file: %v", err)
    }
    testID := "test-job-123"
    if err := SaveAppliedJob(testID); err != nil {
        t.Fatalf("SaveAppliedJob returned error: %v", err)
    }
    jobs, err := LoadAppliedJobs()
    if err != nil {
        t.Fatalf("LoadAppliedJobs returned error: %v", err)
    }
    ts, ok := jobs[testID]
    if !ok {
        t.Fatalf("expected job ID %s in map", testID)
    }
    // Verify the timestamp is a valid RFC3339 time (already parsed).
    if ts.IsZero() {
        t.Fatalf("timestamp for job %s is zero", testID)
    }
    // Cleanup.
    if err := os.Remove(appliedJobsPath); err != nil && !os.IsNotExist(err) {
        t.Fatalf("failed to clean up applied jobs file: %v", err)
    }
    // Ensure the file is removed.
    if _, err := os.Stat(appliedJobsPath); err == nil {
        t.Fatalf("applied jobs file still exists after cleanup")
    }
}
