package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func UpdateEnvFile(contextDir string, key, value string) error {
	envPath := filepath.Join(contextDir, ".env")
	b, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			b = []byte{} // create if not exist
		} else {
			return err
		}
	}

	lines := strings.Split(string(b), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			found = true
			break
		}
	}
	if !found {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return os.WriteFile(envPath, []byte(strings.Join(lines, "\n")), 0o600)
}
