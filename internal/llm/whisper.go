package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

// WhisperClient provides an interface to the OpenAI Whisper API.
type WhisperClient struct {
	APIKey  string
	Model   string
	BaseURL string
}

// NewWhisperClient creates a new Whisper API client.
func NewWhisperClient(apiKey string) *WhisperClient {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return &WhisperClient{
		APIKey:  apiKey,
		Model:   "whisper-1",
		BaseURL: "https://api.openai.com/v1/audio/transcriptions",
	}
}

// Transcribe sends the audio file to Whisper and returns the text transcript.
func (w *WhisperClient) Transcribe(ctx context.Context, fileReader io.Reader, filename string) (string, error) {
	if w.APIKey == "" {
		return "", fmt.Errorf("no OPENAI_API_KEY provided for Whisper")
	}

	var reqBody bytes.Buffer
	writer := multipart.NewWriter(&reqBody)

	// Add file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, fileReader); err != nil {
		return "", err
	}

	// Add model
	if err := writer.WriteField("model", w.Model); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.BaseURL, &reqBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+w.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Text, nil
}
