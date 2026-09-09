package llm

import (
	"testing"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/db"
	"strings"
	"os"
)

func TestGetMemoryContext(t *testing.T) {
	// Use testing DB config
	cfg := &config.Config{
		DBHost:     "localhost",
		DBPort:     "5432",
		DBUser:     "postgres",
		DBPassword: "password",
		DBName:     "djinni_test",
	}
	if os.Getenv("CI") != "" {
		// handle CI if needed
	}
	
	database, err := db.InitDB(cfg)
	if err != nil {
		t.Skip("Skipping db test, no db available")
	}
	defer database.Close()
	
	// Clean table
	database.Exec("DELETE FROM agent_memories")

	// Insert test data
	database.Exec("INSERT INTO agent_memories (category, insight, score) VALUES ('interview', 'Always mention active listening.', 10)")
	database.Exec("INSERT INTO agent_memories (category, insight, score) VALUES ('interview', 'Candidate often forgets SOLID principles.', 5)")

	contextStr := GetMemoryContext(database, "interview")
	if !strings.Contains(contextStr, "Always mention active listening.") {
		t.Errorf("Expected context to contain insight, got: %s", contextStr)
	}
	if !strings.Contains(contextStr, "Candidate often forgets SOLID principles.") {
		t.Errorf("Expected context to contain insight, got: %s", contextStr)
	}
	if !strings.Contains(contextStr, "PAST LESSONS & RULES TO FOLLOW") {
		t.Errorf("Expected context to contain header, got: %s", contextStr)
	}
}
