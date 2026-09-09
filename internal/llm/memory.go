package llm

import (
	"database/sql"
	"fmt"
	"strings"
)

// GetMemoryContext retrieves agent memories for a specific category
// and formats them as a string to be injected into an LLM system prompt.
func GetMemoryContext(db *sql.DB, category string) string {
	if db == nil {
		return ""
	}

	query := `SELECT insight FROM agent_memories WHERE category = $1 ORDER BY score DESC, created_at DESC LIMIT 10`
	rows, err := db.Query(query, category)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var insights []string
	for rows.Next() {
		var insight string
		if err := rows.Scan(&insight); err == nil {
			insights = append(insights, insight)
		}
	}

	if len(insights) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n=== PAST LESSONS & RULES TO FOLLOW ===\n")
	for i, ins := range insights {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, ins))
	}
	sb.WriteString("======================================")

	return sb.String()
}
