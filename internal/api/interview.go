package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"djinni-bot-go/internal/db"
	"djinni-bot-go/internal/llm"
)

// UploadInterviewResponse defines the JSON response structure.
type UploadInterviewResponse struct {
	InterviewID int    `json:"interview_id"`
	Transcript  string `json:"transcript"`
	Summary     string `json:"summary"`
	Status      string `json:"status"`
}

// UploadInterviewHandler handles POST /application/{id}/interview/upload.
// It accepts either a multipart file ("audio") or raw text ("transcript").
func (h *Handlers) UploadInterviewHandler(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, "Database connection not available", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	appID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid application ID", http.StatusBadRequest)
		return
	}

	// Verify application exists
	var exists bool
	err = h.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM applications WHERE id = $1)", appID).Scan(&exists)
	if err != nil || !exists {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	// Try parsing multipart form. Max 100MB to allow audio files.
	err = r.ParseMultipartForm(100 << 20)
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	var transcript string

	// 1. Check for audio file
	file, header, err := r.FormFile("audio")
	if err == nil {
		defer file.Close()
		if h.Whisper == nil {
			http.Error(w, "Whisper client not configured for audio transcription", http.StatusInternalServerError)
			return
		}
		// Transcribe audio
		t, err := h.Whisper.Transcribe(r.Context(), file, header.Filename)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to transcribe audio: %v", err), http.StatusInternalServerError)
			return
		}
		transcript = t
	} else if err != http.ErrMissingFile {
		http.Error(w, "Error reading audio file", http.StatusBadRequest)
		return
	}

	// 2. Fallback to text transcript if audio wasn't provided
	if transcript == "" {
		transcript = r.FormValue("transcript")
	}

	if transcript == "" {
		http.Error(w, "No audio file or transcript provided", http.StatusBadRequest)
		return
	}

	var summary, mistakes, improvements string
	var score int

	type LLMResponse struct {
		Summary      string   `json:"summary"`
		Mistakes     string   `json:"mistakes"`
		Improvements string   `json:"improvements"`
		Score        int      `json:"score"`
		Insights     []string `json:"insights"`
	}

	if h.LLM != nil {
		memoryContext := llm.GetMemoryContext(h.DB, "interview")
		systemPrompt := fmt.Sprintf(`You are an expert technical interviewer and HR assistant. Analyze the following interview transcript.
%s
Extract the following information and output strictly as a JSON object:
{
  "summary": "Key points, questions asked, performance, next steps.",
  "mistakes": "Candidate mistakes, unconfident phrasing, technical gaps.",
  "improvements": "Actionable advice.",
  "score": 85,
  "insights": ["New rule or lesson learned to remember for future analyses"]
}`, memoryContext)

		respText, llmErr := h.LLM.GenerateText(r.Context(), systemPrompt, transcript)
		if llmErr != nil {
			summary = "Failed to generate summary: " + llmErr.Error()
		} else {
			jsonStr := llm.CleanJSON(respText)

			var parsed LLMResponse
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
				summary = parsed.Summary
				mistakes = parsed.Mistakes
				improvements = parsed.Improvements
				score = parsed.Score

				for _, insight := range parsed.Insights {
					mem := &db.AgentMemory{
						Category: "interview",
						Insight:  insight,
						Score:    10,
					}
					db.SaveAgentMemory(h.DB, mem)
				}
			} else {
				summary = respText
			}
		}
	} else {
		summary = "LLM provider not configured."
	}

	var interviewID int
	query := `
		INSERT INTO interviews (application_id, status, transcript, summary, mistakes, improvements, score)
		VALUES ($1, 'completed', $2, $3, $4, $5, $6)
		RETURNING id
	`
	err = h.DB.QueryRow(query, appID, transcript, summary, mistakes, improvements, score).Scan(&interviewID)
	if err != nil {
		http.Error(w, "Failed to save interview", http.StatusInternalServerError)
		return
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadInterviewResponse{
		InterviewID: interviewID,
		Transcript:  transcript,
		Summary:     summary,
		Status:      "completed",
	})
}
