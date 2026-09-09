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
)

func TestUploadInterviewHandler_TextTranscript(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM applications WHERE id = \$1\)`).
		WithArgs(123).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(`INSERT INTO interviews \(application_id, status, transcript, summary\)`).
		WithArgs(123, "Test transcript", "Mocked summary").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	mockLLM := &llm.MockProvider{
		GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
			assert.Equal(t, "Test transcript", user)
			return "Mocked summary", nil
		},
		ProviderName: "TestMockLLM",
	}

	h := NewHandlers(db, mockLLM, nil)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("transcript", "Test transcript")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/application/123/interview/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "123")

	rr := httptest.NewRecorder()
	h.UploadInterviewHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp UploadInterviewResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Equal(t, 1, resp.InterviewID)
	assert.Equal(t, "Test transcript", resp.Transcript)
	assert.Equal(t, "Mocked summary", resp.Summary)
	assert.Equal(t, "completed", resp.Status)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadInterviewHandler_AppNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM applications WHERE id = \$1\)`).
		WithArgs(999).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	h := NewHandlers(db, nil, nil)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	writer.WriteField("transcript", "Test")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/application/999/interview/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "999")

	rr := httptest.NewRecorder()
	h.UploadInterviewHandler(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUploadInterviewHandler_AudioUpload(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM applications WHERE id = \$1\)`).
		WithArgs(123).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	mock.ExpectQuery(`INSERT INTO interviews \(application_id, status, transcript, summary\)`).
		WithArgs(123, "Transcribed audio", "Mocked audio summary").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))

	mockLLM := &llm.MockProvider{
		GenerateTextFunc: func(ctx context.Context, system, user string) (string, error) {
			assert.Equal(t, "Transcribed audio", user)
			return "Mocked audio summary", nil
		},
		ProviderName: "TestMockLLM",
	}

	whisperServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"text":"Transcribed audio"}`))
	}))
	defer whisperServer.Close()

	whisperClient := llm.NewWhisperClient("dummy-key")
	whisperClient.BaseURL = whisperServer.URL

	h := NewHandlers(db, mockLLM, whisperClient)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("audio", "test.mp3")
	assert.NoError(t, err)
	part.Write([]byte("dummy mp3 content"))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/application/123/interview/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetPathValue("id", "123")

	rr := httptest.NewRecorder()
	h.UploadInterviewHandler(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp UploadInterviewResponse
	err = json.Unmarshal(rr.Body.Bytes(), &resp)
	assert.NoError(t, err)

	assert.Equal(t, 2, resp.InterviewID)
	assert.Equal(t, "Transcribed audio", resp.Transcript)
	assert.Equal(t, "Mocked audio summary", resp.Summary)

	assert.NoError(t, mock.ExpectationsWereMet())
}
