package notify

import (
	"testing"
)

func TestTruncateCallbackData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		prefix   string
	}{
		{
			name:   "short callback data remains untouched",
			input:  "apply_accept:short-slug",
			maxLen: 64,
		},
		{
			name:   "exact 64 bytes callback data remains untouched",
			input:  "apply_accept:123456789012345678901234567890123456789012345678901", // 64 bytes
			maxLen: 64,
		},
		{
			name:   "over 64 bytes callback data is truncated to <= 64 bytes",
			input:  "apply_accept:senior-go-developer-with-10-years-experience-in-distributed-systems-and-cloud-native-infrastructure-remote-ukraine-1234567890",
			maxLen: 64,
			prefix: "apply_accept:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateCallbackData(tt.input)
			if len([]byte(got)) > 64 {
				t.Errorf("TruncateCallbackData() output length = %d; want <= 64", len([]byte(got)))
			}
			if tt.prefix != "" && len(got) >= len(tt.prefix) {
				if got[:len(tt.prefix)] != tt.prefix {
					t.Errorf("TruncateCallbackData() lost prefix %q, got %q", tt.prefix, got)
				}
			}
		})
	}
}

func TestSanitizeKeyboard(t *testing.T) {
	longData := "apply_accept:"
	for i := 0; i < 100; i++ {
		longData += "a"
	}

	keyboard := [][]InlineButton{
		{
			{Text: "Submit", CallbackData: longData},
			{Text: "Cancel", CallbackData: "cancel:123"},
		},
	}

	sanitized := SanitizeKeyboard(keyboard)
	if len([]byte(sanitized[0][0].CallbackData)) > 64 {
		t.Errorf("SanitizeKeyboard failed to truncate long callback_data: %d bytes", len([]byte(sanitized[0][0].CallbackData)))
	}
	if sanitized[0][1].CallbackData != "cancel:123" {
		t.Errorf("SanitizeKeyboard mutated short callback_data: %s", sanitized[0][1].CallbackData)
	}
}
