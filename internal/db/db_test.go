package db

import (
	"regexp"
	"testing"

	"djinni-bot-go/internal/config"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestRunMigrations(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS applications`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS interviews`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS chat_logs`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`CREATE TABLE IF NOT EXISTS statuses`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS transcript`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta(`ALTER TABLE interviews ADD COLUMN IF NOT EXISTS summary`)).WillReturnResult(sqlmock.NewResult(0, 0))

	err = runMigrations(db)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	app := &Application{
		JobID:       "123",
		CompanyName: "Test Co",
		Status:      "Applied",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO applications (job_id, company_name, status) VALUES ($1, $2, $3) RETURNING id`)).
		WithArgs(app.JobID, app.CompanyName, app.Status).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err = CreateApplication(db, app)
	assert.NoError(t, err)
	assert.Equal(t, 1, app.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetApplication(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "job_id", "company_name", "status", "created_at"}).
		AddRow(1, "123", "Test Co", "Applied", "2023-01-01 00:00:00")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, job_id, company_name, status, created_at FROM applications WHERE id = $1`)).
		WithArgs(1).
		WillReturnRows(rows)

	app, err := GetApplication(db, 1)
	assert.NoError(t, err)
	assert.Equal(t, "123", app.JobID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateApplicationStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE applications SET status = $1 WHERE id = $2`)).
		WithArgs("Rejected", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = UpdateApplicationStatus(db, 1, "Rejected")
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateInterview(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	interview := &Interview{
		ApplicationID: 1,
		ScheduledAt:   "2023-01-01 10:00:00",
		Status:        "Scheduled",
		Transcript:    "Transcript",
		Summary:       "Summary",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO interviews (application_id, scheduled_at, status, transcript, summary) VALUES ($1, $2, $3, $4, $5) RETURNING id`)).
		WithArgs(interview.ApplicationID, interview.ScheduledAt, interview.Status, interview.Transcript, interview.Summary).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err = CreateInterview(db, interview)
	assert.NoError(t, err)
	assert.Equal(t, 1, interview.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateChatLog(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	chatLog := &ChatLog{
		ApplicationID: 1,
		Message:       "Hello",
		Sender:        "User",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO chat_logs (application_id, message, sender) VALUES ($1, $2, $3) RETURNING id`)).
		WithArgs(chatLog.ApplicationID, chatLog.Message, chatLog.Sender).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	err = CreateChatLog(db, chatLog)
	assert.NoError(t, err)
	assert.Equal(t, 1, chatLog.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestInitDB(t *testing.T) {
	// Not practically testable fully without actual Postgres since sql.Open doesn't hit sqlmock without custom driver registration.
	// But we can test it returns an error with invalid DB configs to increase coverage somewhat.
	cfg := &config.Config{
		DBHost:     "invalid_host",
		DBPort:     "0",
		DBUser:     "user",
		DBPassword: "password",
		DBName:     "db",
	}
	db, err := InitDB(cfg)
	assert.Error(t, err)
	if db != nil {
		db.Close()
	}
}
