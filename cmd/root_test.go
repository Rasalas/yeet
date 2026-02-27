package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestColorizeDiffStat(t *testing.T) {
	t.Run("file with insertions and deletions", func(t *testing.T) {
		line := " cmd/root.go | 29 +++---"
		got := colorizeDiffStat(line)

		if !strings.Contains(got, green+"+++"+reset) {
			t.Errorf("missing green insertions in %q", got)
		}
		if !strings.Contains(got, red+"---"+reset) {
			t.Errorf("missing red deletions in %q", got)
		}
	})

	t.Run("file with insertions only", func(t *testing.T) {
		line := " stream.go | 45 +++++++++++++"
		got := colorizeDiffStat(line)

		if !strings.Contains(got, green) {
			t.Error("missing green for insertions")
		}
		if strings.Contains(got, red) {
			t.Error("should not contain red for insertions-only line")
		}
	})

	t.Run("summary line", func(t *testing.T) {
		line := " 5 files changed, 428 insertions(+), 19 deletions(-)"
		got := colorizeDiffStat(line)

		if !strings.HasPrefix(got, dim) {
			t.Errorf("summary line should start with dim: %q", got)
		}
		if !strings.HasSuffix(got, reset) {
			t.Errorf("summary line should end with reset: %q", got)
		}
	})

	t.Run("single file changed", func(t *testing.T) {
		line := " 1 file changed, 3 insertions(+)"
		got := colorizeDiffStat(line)

		if !strings.HasPrefix(got, dim) {
			t.Error("single file summary should be dim")
		}
	})

	t.Run("unrecognized line passthrough", func(t *testing.T) {
		line := "some random text"
		got := colorizeDiffStat(line)
		if got != line {
			t.Errorf("unrecognized line should pass through unchanged, got %q", got)
		}
	})
}

func TestColorizeDiffStatNoColor(t *testing.T) {
	// Save originals
	origBold, origDim, origRed, origGreen, origReset := bold, dim, red, green, reset
	bold, dim, red, green, reset = "", "", "", "", ""
	defer func() { bold, dim, red, green, reset = origBold, origDim, origRed, origGreen, origReset }()

	line := " cmd/root.go | 29 +++---"
	got := colorizeDiffStat(line)

	// With NO_COLOR, should contain no escape codes
	if strings.Contains(got, "\033") {
		t.Errorf("NO_COLOR output contains escape codes: %q", got)
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"single line", "single line"},
		{"first\nsecond\nthird", "first"},
		{"", ""},
		{"\nleading newline", ""},
		{"trailing newline\n", "trailing newline"},
	}

	for _, tt := range tests {
		got := firstLine(tt.input)
		if got != tt.want {
			t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetEditor(t *testing.T) {
	// Save and clear env
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
	}()

	t.Run("VISUAL takes precedence", func(t *testing.T) {
		os.Setenv("VISUAL", "code")
		os.Setenv("EDITOR", "vim")
		if got := getEditor(); got != "code" {
			t.Errorf("getEditor() = %q, want \"code\"", got)
		}
	})

	t.Run("EDITOR fallback", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Setenv("EDITOR", "nano")
		if got := getEditor(); got != "nano" {
			t.Errorf("getEditor() = %q, want \"nano\"", got)
		}
	})

	t.Run("default vi", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")
		if got := getEditor(); got != "vi" {
			t.Errorf("getEditor() = %q, want \"vi\"", got)
		}
	})
}
