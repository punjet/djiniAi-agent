package api

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"djinni-bot-go/internal/llm"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadChatLogHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT job_id, company_name, status FROM applications WHERE id = \$1`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "company_name", "status"}).AddRow("job1", "Acme", "applied"))

	mock.ExpectQuery(`INSERT INTO chat_logs \(application_id, message, sender\)`).
		WithArgs(1, "Recruiter: Hey\nCandidate: tomorrow.").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))

	mock.ExpectQuery(`SELECT insight FROM agent_memories WHERE category = \$1`).
		WithArgs("chatlog").
		WillReturnRows(sqlmock.NewRows([]string{"insight"}).AddRow("Be nice"))

	mock.ExpectExec(`UPDATE applications SET status = \$1 WHERE id = \$2`).
		WithArgs("technical_interview", 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mockLLM := &llm.MockProvider{
		GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
			return "technical_interview", nil
		},
	}

	h := NewHandlers(db, mockLLM, nil)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("chat_log", "Recruiter: Hey\nCandidate: tomorrow.")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/application/1/chat-log", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "1")

	rr := httptest.NewRecorder()
	h.UploadChatLogHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp UploadChatLogResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "technical_interview", resp.NewStatus)
	assert.Equal(t, 10, resp.ChatLogID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadChatLogHandler_NoChange(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT job_id, company_name, status FROM applications WHERE id = \$1`).
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"job_id", "company_name", "status"}).AddRow("job2", "Beta", "applied"))

	mock.ExpectQuery(`INSERT INTO chat_logs \(application_id, message, sender\)`).
		WithArgs(1, "Recruiter: hi").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))

	mock.ExpectQuery(`SELECT insight FROM agent_memories WHERE category = \$1`).
		WithArgs("chatlog").
		WillReturnRows(sqlmock.NewRows([]string{"insight"}).AddRow("Be nice"))

	// Should not expect an UPDATE here

	mockLLM := &llm.MockProvider{
		GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
			return "NO_CHANGE", nil
		},
	}

	h := NewHandlers(db, mockLLM, nil)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("chat_log", "Recruiter: hi")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/application/1/chat-log", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "1")

	rr := httptest.NewRecorder()
	h.UploadChatLogHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp UploadChatLogResponse
	err = json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "", resp.NewStatus)
	assert.Equal(t, 11, resp.ChatLogID)

	require.NoError(t, mock.ExpectationsWereMet())
}
