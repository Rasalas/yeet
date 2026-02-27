package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestParsePR(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "title and body",
			input:     "Add user authentication\n\n## Summary\n- Added login flow\n- Added logout",
			wantTitle: "Add user authentication",
			wantBody:  "## Summary\n- Added login flow\n- Added logout",
		},
		{
			name:      "title only",
			input:     "Fix typo in README",
			wantTitle: "Fix typo in README",
			wantBody:  "",
		},
		{
			name:      "leading and trailing whitespace",
			input:     "  Fix bug  \n\n  Body text  ",
			wantTitle: "Fix bug",
			wantBody:  "Body text",
		},
		{
			name:      "empty input",
			input:     "",
			wantTitle: "",
			wantBody:  "",
		},
		{
			name:      "only whitespace",
			input:     "   \n\n   ",
			wantTitle: "",
			wantBody:  "",
		},
		{
			name:      "title with single newline body",
			input:     "Title\nBody line",
			wantTitle: "Title",
			wantBody:  "Body line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := parsePR(tt.input)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestDisplayPRPreview(t *testing.T) {
	// Set NO_COLOR to get predictable output
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	// Capture that it doesn't panic and returns a reasonable line count
	lines := displayPRPreview("Test Title", "## Summary\n- Change one")

	if lines < 3 {
		t.Errorf("displayPRPreview returned %d lines, expected at least 3", lines)
	}
}

func TestParsePRRoundTrip(t *testing.T) {
	original := "Implement dark mode\n\n## Summary\n- Added theme toggle\n- Updated CSS variables"
	title, body := parsePR(original)

	reconstructed := title + "\n\n" + body
	if strings.TrimSpace(reconstructed) != strings.TrimSpace(original) {
		t.Errorf("round-trip failed:\n  got:  %q\n  want: %q", reconstructed, original)
	}
}
