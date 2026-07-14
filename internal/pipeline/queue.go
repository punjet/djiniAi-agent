package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type PendingApplication struct {
	JobSlug       string            `json:"job_slug"`
	Message       string            `json:"message"`
	CVFileName    string            `json:"cv_file_name"`
	CVPath        string            `json:"cv_path"` // Path to saved CV file
	ExtraFormData map[string]string `json:"extra_form_data"`
}

var queueMutex sync.Mutex

func SavePendingApplication(contextDir string, app PendingApplication, cvBytes []byte) error {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	dataDir := filepath.Join(contextDir, "data")
	os.MkdirAll(dataDir, 0o755)

	cvsDir := filepath.Join(dataDir, "pending_cvs")
	os.MkdirAll(cvsDir, 0o755)

	if len(cvBytes) > 0 && app.CVFileName != "" {
		cvPath := filepath.Join(cvsDir, app.CVFileName)
		if err := os.WriteFile(cvPath, cvBytes, 0o644); err != nil {
			return fmt.Errorf("failed to save pending CV: %w", err)
		}
		app.CVPath = cvPath
	}

	queueFile := filepath.Join(dataDir, "pending_applications.jsonl")
	f, err := os.OpenFile(queueFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(app)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func LoadPendingApplications(contextDir string) ([]PendingApplication, error) {
	queueMutex.Lock()
	defer queueMutex.Unlock()

	queueFile := filepath.Join(contextDir, "data", "pending_applications.jsonl")
	b, err := os.ReadFile(queueFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var apps []PendingApplication
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var app PendingApplication
		if err := json.Unmarshal([]byte(line), &app); err != nil {
			continue // skip invalid lines
		}
		apps = append(apps, app)
	}
	return apps, nil
}

func ClearPendingApplications(contextDir string) error {
	queueMutex.Lock()
	defer queueMutex.Unlock()
	queueFile := filepath.Join(contextDir, "data", "pending_applications.jsonl")
	err := os.Remove(queueFile)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
