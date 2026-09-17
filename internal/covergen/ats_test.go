package covergen

import (
	"testing"
)

func TestNormalizeForATS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"em dash", "5\u20146", "5-6"},
		{"en dash", "2019\u20132024", "2019-2024"},
		{"figure dash", "item\u20124", "item-4"},
		{"horizontal bar", "note\u2015end", "note-end"},
		{"smart left single quote", "\u2018hello\u2019", "'hello'"},
		{"smart right single quote", "it\u2019s", "it's"},
		{"low-9 single quote", "\u201Aquote", "'quote"},
		{"smart left double quote", "\u201Chello\u201D", "\"hello\""},
		{"low-9 double quote", "\u201Eword\u201D", "\"word\""},
		{"angle double left", "\u00ABword\u00BB", "\"word\""},
		{"nbsp", "hello\u00A0world", "hello world"},
		{"narrow nbsp", "hello\u202Fworld", "hello world"},
		{"thin space", "hello\u2009world", "hello world"},
		{"em space", "hello\u2003world", "hello world"},
		{"ellipsis", "wait\u2026", "wait..."},
		{"bullet", "\u2022 item", "- item"},
		{"triangular bullet", "\u2023 item", "- item"},
		{"white bullet", "\u25E6 item", "- item"},
		{"zero width space removed", "he\u200Bllo", "hello"},
		{"zwnj removed", "he\u200Cllo", "hello"},
		{"zwj removed", "he\u200Dllo", "hello"},
		{"bom removed", "\uFEFFstart", "start"},
		{"soft hyphen removed", "hyphen\u00ADation", "hyphenation"},
		{"plain ascii unchanged", "Hello World 2024", "Hello World 2024"},
		{"html tags preserved", "<p>Hello\u2014World</p>", "<p>Hello-World</p>"},
		{"mixed replacements", "\u201CHello\u201D \u2013 it\u2019s \u2026 \u2022 done", "\"Hello\" - it's ... - done"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeForATS(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeForATS(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
