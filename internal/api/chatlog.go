package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"djinni-bot-go/internal/llm"
)

type UploadChatLogResponse struct {
	ChatLogID int    `json:"chat_log_id"`
	NewStatus string `json:"new_status,omitempty"`
}

func (h *Handlers) UploadChatLogHandler(w http.ResponseWriter, r *http.Request) {
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

	// Fetch application info for context
	var jobID, companyName, currentStatus string
	err = h.DB.QueryRow("SELECT job_id, company_name, status FROM applications WHERE id = $1", appID).
		Scan(&jobID, &companyName, &currentStatus)
	if err != nil {
		http.Error(w, "Application not found", http.StatusNotFound)
		return
	}

	err = r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil && err != http.ErrNotMultipart {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	var chatLog string
	file, _, err := r.FormFile("chat_log")
	if err == nil {
		defer file.Close()
		content, err := io.ReadAll(file)
		if err == nil {
			chatLog = string(content)
		}
	} else {
		chatLog = r.FormValue("chat_log")
	}

	if strings.TrimSpace(chatLog) == "" {
		http.Error(w, "chat_log is required", http.StatusBadRequest)
		return
	}

	// Insert into chat_logs
	var chatLogID int
	err = h.DB.QueryRow(`
		INSERT INTO chat_logs (application_id, message, sender)
		VALUES ($1, $2, 'telegram')
		RETURNING id
	`, appID, chatLog).Scan(&chatLogID)
	if err != nil {
		http.Error(w, "Failed to save chat log", http.StatusInternalServerError)
		return
	}

	// Evaluate new status using LLM
	newStatus := ""
	if h.LLM != nil {
		memoryContext := llm.GetMemoryContext(h.DB, "chatlog")
		systemPrompt := fmt.Sprintf(`You are an HR recruitment assistant analyzing a chat log for an application.
Job ID: %s
Company: %s
Current Status: %s%s

Determine the NEW application status based on this chat log.
Allowed statuses: applied, recruiter_contact, technical_interview, rejected, offer.

Respond ONLY with the exact status name if it changes, or "NO_CHANGE" if it remains the same. Do not include any other text.`, jobID, companyName, currentStatus, memoryContext)

		resp, err := h.LLM.GenerateText(r.Context(), systemPrompt, chatLog)
		if err == nil {
			respStr := strings.TrimSpace(resp)
			validStatuses := map[string]bool{
				"applied":             true,
				"recruiter_contact":   true,
				"technical_interview": true,
				"rejected":            true,
				"offer":               true,
			}
			if validStatuses[respStr] && respStr != currentStatus {
				newStatus = respStr
				// Update status in db
				_, err = h.DB.Exec("UPDATE applications SET status = $1 WHERE id = $2", newStatus, appID)
				if err != nil {
					// could not update status, just ignore or log
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UploadChatLogResponse{
		ChatLogID: chatLogID,
		NewStatus: newStatus,
	})
}
