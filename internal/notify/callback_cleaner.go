package notify

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const maxCallbackDataBytes = 64

// TruncateCallbackData ensures that a callback_data string is strictly <= 64 bytes
// (Telegram API limit for inline button callback_data).
// If the length exceeds 64 bytes, it truncates the string and appends a short SHA256 hash
// of the original string to maintain uniqueness.
func TruncateCallbackData(data string) string {
	if len([]byte(data)) <= maxCallbackDataBytes {
		return data
	}

	// Calculate SHA256 hash of the full data string
	hash := sha256.Sum256([]byte(data))
	hashHex := hex.EncodeToString(hash[:])[:8] // 8 chars of hash

	// If data has a prefix:val structure (e.g., "apply_accept:very_long_slug..."), preserve prefix
	prefix := ""
	if idx := strings.Index(data, ":"); idx != -1 && idx < 20 {
		prefix = data[:idx+1]
	}

	suffix := "_" + hashHex // e.g. "_a1b2c3d4" (9 bytes)
	allowedLen := maxCallbackDataBytes - len(suffix)

	if len(prefix) > 0 && allowedLen > len(prefix) {
		remaining := allowedLen - len(prefix)
		val := data[len(prefix):]
		if len(val) > remaining {
			val = val[:remaining]
		}
		return prefix + val + suffix
	}

	// Fallback simple truncation with hash suffix
	return data[:allowedLen] + suffix
}

// SanitizeKeyboard iterates through all buttons in a keyboard grid and truncates
// any CallbackData exceeding 64 bytes.
func SanitizeKeyboard(keyboard [][]InlineButton) [][]InlineButton {
	if keyboard == nil {
		return nil
	}
	sanitized := make([][]InlineButton, len(keyboard))
	for i, row := range keyboard {
		sanitizedRow := make([]InlineButton, len(row))
		for j, btn := range row {
			btn.CallbackData = TruncateCallbackData(btn.CallbackData)
			sanitizedRow[j] = btn
		}
		sanitized[i] = sanitizedRow
	}
	return sanitized
}
