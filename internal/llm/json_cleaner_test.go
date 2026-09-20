package llm

import (
	"encoding/json"
	"testing"
)

func TestCleanJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{
			name:     "clean json intact",
			input:    `{"key": "value", "number": 123}`,
			expected: `{"key": "value", "number": 123}`,
			valid:    true,
		},
		{
			name: "markdown json block",
			input: "```json\n" +
				"{\n" +
				`  "key": "value"` + "\n" +
				"}\n" +
				"```",
			expected: `{"key": "value"}`,
			valid:    true,
		},
		{
			name: "trailing commas in object and array",
			input: `
			{
				"letter": {
					"greeting": "Hello,",
					"closing": "Regards",
				},
				"achievements": [
					"item 1",
					"item 2",
				],
			}
			`,
			expected: `{"letter": {"greeting": "Hello,","closing": "Regards"},"achievements": ["item 1","item 2"]}`,
			valid:    true,
		},
		{
			name:     "string with comma before quote inside string",
			input:    `{"text": "hello, world,", "number": 1}`,
			expected: `{"text": "hello, world,", "number": 1}`,
			valid:    true,
		},
		{
			name:     "string with trailing comma pattern inside string literal",
			input:    `{"text": "value with ,}", "valid": true,}`,
			expected: `{"text": "value with ,}", "valid": true}`,
			valid:    true,
		},
		{
			name:     "markdown wrapper with extra text",
			input:    "Here is your JSON response:\n```json\n{\"greeting\": \"Hi\"}\n```\nHope this helps!",
			expected: `{"greeting": "Hi"}`,
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanJSON(tt.input)
			if tt.valid {
				if !json.Valid([]byte(got)) {
					t.Errorf("CleanJSON() output is not valid JSON: %s", got)
				}
			}
		})
	}
}
