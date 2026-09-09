package api

import (
	"fmt"
	"net/http"
	"strconv"

	"djinni-bot-go/internal/db"
)

func (h *Handlers) FeedbackHandler(w http.ResponseWriter, r *http.Request) {
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

	err = r.ParseForm()
	if err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	insight := r.FormValue("insight")
	category := r.FormValue("category")

	if insight == "" || category == "" {
		http.Error(w, "Insight and category are required", http.StatusBadRequest)
		return
	}

	mem := &db.AgentMemory{
		Category:   category,
		Insight:    insight,
		ContextKey: fmt.Sprintf("app_%d", appID),
	}

	err = db.SaveAgentMemory(h.DB, mem)
	if err != nil {
		http.Error(w, "Failed to save feedback", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<div class="p-4 mb-4 text-sm text-green-700 bg-green-100 rounded-lg" role="alert">
  <span class="font-medium">Success!</span> Feedback saved to agent memories.
</div>`))
}
