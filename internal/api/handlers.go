package api

import (
	"database/sql"
	"html/template"
	"net/http"
	"strconv"

	"djinni-bot-go/internal/llm"
)

type Application struct {
	ID          int
	JobID       string
	CompanyName string
	Status      string
}

type Handlers struct {
	DB        *sql.DB
	Templates *template.Template
	LLM       llm.Provider
	Whisper   *llm.WhisperClient
}

func NewHandlers(db *sql.DB, llmProvider llm.Provider, whisperClient *llm.WhisperClient) *Handlers {
	return &Handlers{
		DB:      db,
		LLM:     llmProvider,
		Whisper: whisperClient,
	}
}

func (h *Handlers) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query("SELECT id, job_id, company_name, status FROM applications ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "Failed to fetch applications", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var apps []Application
	for rows.Next() {
		var app Application
		if err := rows.Scan(&app.ID, &app.JobID, &app.CompanyName, &app.Status); err != nil {
			continue
		}
		apps = append(apps, app)
	}

	data := struct {
		Applications []Application
	}{
		Applications: apps,
	}

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/dashboard.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	// Render layout with dashboard content
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) ApplicationDetailHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	var app Application
	err = h.DB.QueryRow("SELECT id, job_id, company_name, status FROM applications WHERE id = $1", id).
		Scan(&app.ID, &app.JobID, &app.CompanyName, &app.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Application not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var score int
	var mistakes, improvements string
	err = h.DB.QueryRow("SELECT score, mistakes, improvements FROM interviews WHERE application_id = $1 ORDER BY created_at DESC LIMIT 1", id).
		Scan(&score, &mistakes, &improvements)
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Application  Application
		Score        int
		Mistakes     string
		Improvements string
	}{
		Application:  app,
		Score:        score,
		Mistakes:     mistakes,
		Improvements: improvements,
	}

	// We need to render the layout, but pass application_detail content. 
	// To do this simply, we re-parse or use block template tricks.
	// Since layout.html defines content via `{{template "content" .}}`
	// We need to execute layout.html, but how does it know which "content" to use?
	// It's better to parse layout + specific template per handler if we use `define "content"`.

	tmpl, err := template.ParseFiles("templates/layout.html", "templates/application_detail.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) UpdateStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	newStatus := r.FormValue("status")
	if newStatus == "" {
		http.Error(w, "Status is required", http.StatusBadRequest)
		return
	}

	_, err = h.DB.Exec("UPDATE applications SET status = $1 WHERE id = $2", newStatus, id)
	if err != nil {
		http.Error(w, "Failed to update status", http.StatusInternalServerError)
		return
	}

	// Render just the status badge fragment
	tmpl, err := template.ParseFiles("templates/application_detail.html")
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	app := Application{ID: id, Status: newStatus}
	if err := tmpl.ExecuteTemplate(w, "status_badge", app); err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard", h.DashboardHandler)
	mux.HandleFunc("GET /application/{id}", h.ApplicationDetailHandler)
	mux.HandleFunc("POST /application/{id}/status", h.UpdateStatusHandler)
	mux.HandleFunc("POST /application/{id}/feedback", h.FeedbackHandler)
	mux.HandleFunc("POST /application/{id}/interview/upload", h.UploadInterviewHandler)
	mux.HandleFunc("POST /application/{id}/chat-log", h.UploadChatLogHandler)
}
