package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func SendDocument(filename string, fileData []byte, caption string) (int64, error) {
	token := os.Getenv("TG_BOT_TOKEN")
	chatID := os.Getenv("TG_CHAT_ID")

	if token == "" || chatID == "" {
		return 0, fmt.Errorf("telegram TG_BOT_TOKEN or TG_CHAT_ID missing")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", token)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add chat_id
	_ = writer.WriteField("chat_id", chatID)
	// Add caption
	if caption != "" {
		_ = writer.WriteField("caption", caption)
	}

	// Add document
	part, err := writer.CreateFormFile("document", filepath.Base(filename))
	if err != nil {
		return 0, err
	}
	_, err = part.Write(fileData)
	if err != nil {
		return 0, err
	}
	err = writer.Close()
	if err != nil {
		return 0, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("telegram document request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var tgResp tgResponse
	if err := json.Unmarshal(respBytes, &tgResp); err != nil {
		return 0, fmt.Errorf("failed to parse sendDocument response: %w, body: %s", err, string(respBytes))
	}

	if !tgResp.OK {
		return 0, fmt.Errorf("telegram API returned OK=false: %s (code %d)", tgResp.Description, tgResp.ErrorCode)
	}

	var msgResult tgSendMessageResult
	if err := json.Unmarshal(tgResp.Result, &msgResult); err != nil {
		return 0, err
	}

	return msgResult.MessageID, nil
}
