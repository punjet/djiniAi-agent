package db

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	_ "github.com/lib/pq"
	"djinni-bot-go/internal/config"
)

// InitDB initializes the database connection and runs migrations.
func InitDB(cfg *config.Config) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	
	if err := db.Ping(); err != nil {
		return nil, err
	}
	
	if err := runMigrations(db); err != nil {
		return nil, err
	}
	
	return db, nil
}

func runMigrations(db *sql.DB) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS vector;`,
		`CREATE TABLE IF NOT EXISTS wiki_documents (
			id SERIAL PRIMARY KEY,
			url TEXT,
			title TEXT,
			file_path TEXT,
			embedding vector(1536)
		);`,
		`CREATE TABLE IF NOT EXISTS applications (
			id SERIAL PRIMARY KEY,
			job_id VARCHAR(255) UNIQUE NOT NULL,
			company_name VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS interviews (
			id SERIAL PRIMARY KEY,
			application_id INT REFERENCES applications(id),
			scheduled_at TIMESTAMP,
			status VARCHAR(50) NOT NULL,
			transcript TEXT,
			summary TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS chat_logs (
			id SERIAL PRIMARY KEY,
			application_id INT REFERENCES applications(id),
			message TEXT NOT NULL,
			sender VARCHAR(50) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS statuses (
			id SERIAL PRIMARY KEY,
			name VARCHAR(50) UNIQUE NOT NULL
		);`,
		`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS transcript TEXT;`,
		`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS summary TEXT;`,
		`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS mistakes TEXT;`,
		`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS improvements TEXT;`,
		`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS score INT DEFAULT 0;`,
		`CREATE TABLE IF NOT EXISTS agent_memories (
			id SERIAL PRIMARY KEY,
			category VARCHAR(50),
			insight TEXT,
			context_key VARCHAR(100),
			score INT DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);`,
		`ALTER TABLE applications ADD CONSTRAINT applications_job_id_key UNIQUE (job_id);`,
	}
	
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			if strings.Contains(q, "ADD CONSTRAINT") && strings.Contains(err.Error(), "already exists") {
				continue
			}
			log.Printf("Failed to run migration: %s\nError: %v", q, err)
			return err
		}
	}
	
	return nil
}

// Application represents a job application
type Application struct {
	ID          int
	JobID       string
	CompanyName string
	Status      string
	CreatedAt   string
}

// Interview represents an interview
type Interview struct {
	ID            int
	ApplicationID int
	ScheduledAt   string
	Status        string
	Transcript    string
	Summary       string
	Mistakes      string
	Improvements  string
	Score         int
	CreatedAt     string
}

// AgentMemory represents learned insights from the LLM agent
type AgentMemory struct {
	ID         int
	Category   string
	Insight    string
	ContextKey string
	Score      int
	CreatedAt  string
}

// ChatLog represents a chat log
type ChatLog struct {
	ID            int
	ApplicationID int
	Message       string
	Sender        string
	CreatedAt     string
}

func CreateApplication(db *sql.DB, app *Application) error {
	query := `INSERT INTO applications (job_id, company_name, status) VALUES ($1, $2, $3) RETURNING id`
	return db.QueryRow(query, app.JobID, app.CompanyName, app.Status).Scan(&app.ID)
}

func GetApplication(db *sql.DB, id int) (*Application, error) {
	app := &Application{}
	query := `SELECT id, job_id, company_name, status, created_at FROM applications WHERE id = $1`
	err := db.QueryRow(query, id).Scan(&app.ID, &app.JobID, &app.CompanyName, &app.Status, &app.CreatedAt)
	if err != nil {
		return nil, err
	}
	return app, nil
}

func UpdateApplicationStatus(db *sql.DB, id int, status string) error {
	query := `UPDATE applications SET status = $1 WHERE id = $2`
	_, err := db.Exec(query, status, id)
	return err
}

func CreateInterview(db *sql.DB, interview *Interview) error {
	query := `INSERT INTO interviews (application_id, scheduled_at, status, transcript, summary) VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return db.QueryRow(query, interview.ApplicationID, interview.ScheduledAt, interview.Status, interview.Transcript, interview.Summary).Scan(&interview.ID)
}

func CreateChatLog(db *sql.DB, log *ChatLog) error {
	query := `INSERT INTO chat_logs (application_id, message, sender) VALUES ($1, $2, $3) RETURNING id`
	return db.QueryRow(query, log.ApplicationID, log.Message, log.Sender).Scan(&log.ID)
}

func UpdateInterviewCoaching(db *sql.DB, id int, mistakes, improvements string, score int) error {
	query := `UPDATE interviews SET mistakes = $1, improvements = $2, score = $3 WHERE id = $4`
	_, err := db.Exec(query, mistakes, improvements, score, id)
	return err
}

func SaveAgentMemory(db *sql.DB, mem *AgentMemory) error {
	query := `INSERT INTO agent_memories (category, insight, context_key, score) VALUES ($1, $2, $3, $4) RETURNING id`
	return db.QueryRow(query, mem.Category, mem.Insight, mem.ContextKey, mem.Score).Scan(&mem.ID)
}

func GetAgentMemories(db *sql.DB, category, contextKey string) ([]AgentMemory, error) {
	query := `SELECT id, category, insight, context_key, score, created_at FROM agent_memories WHERE category = $1 AND context_key = $2 ORDER BY score DESC`
	rows, err := db.Query(query, category, contextKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []AgentMemory
	for rows.Next() {
		var mem AgentMemory
		if err := rows.Scan(&mem.ID, &mem.Category, &mem.Insight, &mem.ContextKey, &mem.Score, &mem.CreatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, mem)
	}
	return memories, rows.Err()
}

func cleanURLPath(u string) string {
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

func normalizeDBString(s string) string {
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[^\p{L}\p{N}\s]`)
	s = reg.ReplaceAllString(s, "")
	return strings.Join(strings.Fields(s), " ")
}

func MigrateFilesToDB(db *sql.DB, contextDir string) error {
	if db == nil {
		return nil
	}

	historyPath := filepath.Join(contextDir, "data", "scan-history.tsv")
	appsPath := filepath.Join(contextDir, "data", "applications.md")

	query := `INSERT INTO applications (job_id, company_name, status) VALUES ($1, $2, $3) ON CONFLICT (job_id) DO NOTHING`

	if file, err := os.Open(historyPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		if scanner.Scan() {
			for scanner.Scan() {
				line := scanner.Text()
				parts := strings.Split(line, "\t")
				if len(parts) > 0 && parts[0] != "" {
					jobURL := cleanURLPath(parts[0])
					company := "Unknown"
					if len(parts) >= 5 && parts[4] != "" {
						company = parts[4]
					}
					status := "scanned"
					if len(parts) >= 6 && parts[5] != "" {
						status = parts[5]
					}
					if _, err := db.Exec(query, jobURL, company, status); err != nil {
						log.Printf("Failed to insert scan history job %s into DB: %v", jobURL, err)
					}
				}
			}
		}
	}

	if file, err := os.Open(appsPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		urlRx := regexp.MustCompile(`https?://[^\s|)]+`)
		rowRx := regexp.MustCompile(`\|[^|]+\|[^|]+\|\s*([^|]+)\s*\|\s*([^|]+)\s*\|`)

		for scanner.Scan() {
			line := scanner.Text()
			company := "Unknown"
			if matches := rowRx.FindStringSubmatch(line); len(matches) > 2 {
				c := strings.TrimSpace(matches[1])
				normC := normalizeDBString(c)
				if normC != "" && normC != "company" && normC != "empresa" {
					company = c
				}
			}

			urls := urlRx.FindAllString(line, -1)
			for _, u := range urls {
				jobURL := cleanURLPath(u)
				if _, err := db.Exec(query, jobURL, company, "applied"); err != nil {
					log.Printf("Failed to insert markdown application job %s into DB: %v", jobURL, err)
				}
			}
		}
	}

	return nil
}
