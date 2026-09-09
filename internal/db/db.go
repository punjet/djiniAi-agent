package db

import (
	"database/sql"
	"fmt"
	"log"

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
		`CREATE TABLE IF NOT EXISTS applications (
			id SERIAL PRIMARY KEY,
			job_id VARCHAR(255) NOT NULL,
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
	}
	
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
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
