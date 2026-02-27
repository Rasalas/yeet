package term

import (
	"os"
	"testing"
)

func TestGetEditor(t *testing.T) {
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	defer func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
	}()

	t.Run("VISUAL takes precedence", func(t *testing.T) {
		os.Setenv("VISUAL", "code")
		os.Setenv("EDITOR", "vim")
		if got := GetEditor(); got != "code" {
			t.Errorf("GetEditor() = %q, want \"code\"", got)
		}
	})

	t.Run("EDITOR fallback", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Setenv("EDITOR", "nano")
		if got := GetEditor(); got != "nano" {
			t.Errorf("GetEditor() = %q, want \"nano\"", got)
		}
	})

	t.Run("default vi", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")
		if got := GetEditor(); got != "vi" {
			t.Errorf("GetEditor() = %q, want \"vi\"", got)
		}
	})
}
