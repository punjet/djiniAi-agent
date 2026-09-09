package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFeedbackHandler(t *testing.T) {
	dbMock, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer dbMock.Close()

	h := &Handlers{
		DB: dbMock,
	}

	mock.ExpectQuery(`INSERT INTO agent_memories \(category, insight, context_key, score\) VALUES \(\$1, \$2, \$3, \$4\) RETURNING id`).
		WithArgs("Communication", "Candidate should speak slower", "app_1", 0).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /application/{id}/feedback", h.FeedbackHandler)

	form := url.Values{}
	form.Add("insight", "Candidate should speak slower")
	form.Add("category", "Communication")

	req, err := http.NewRequest("POST", "/application/1/feedback", bytes.NewBufferString(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if !bytes.Contains(rr.Body.Bytes(), []byte("Success!")) {
		t.Errorf("handler returned unexpected body: got %v", rr.Body.String())
	}
}
