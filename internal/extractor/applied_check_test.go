package extractor

import (
	"testing"
)

func TestIsAlreadyApplied(t *testing.T) {
	tests := []struct {
		name string
		html string
		want bool
	}{
		{
			name: "Standard apostrophe applied message",
			html: `<div class="card card-body">You've applied to this job already.</div>`,
			want: true,
		},
		{
			name: "Curly apostrophe applied message",
			html: `<div class="card card-body">You’ve applied to this job already.</div>`,
			want: true,
		},
		{
			name: "No applied message - normal HTML",
			html: `<div><button class="btn btn-primary">Apply</button></div>`,
			want: false,
		},
		{
			name: "Empty HTML",
			html: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAlreadyApplied(tt.html)
			if got != tt.want {
				t.Errorf("IsAlreadyApplied() = %v, want %v", got, tt.want)
			}
		})
	}
}
