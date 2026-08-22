package term

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDecodeEditEvent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantEv   editEvent
		wantRune rune
		wantSize int
	}{
		{name: "enter", input: "\r", wantEv: editEnter, wantSize: 1},
		{name: "newline", input: "\n", wantEv: editEnter, wantSize: 1},
		{name: "ctrl-c aborts", input: "\x03", wantEv: editAbort, wantSize: 1},
		{name: "backspace", input: "\x7f", wantEv: editBackspace, wantSize: 1},
		{name: "ctrl-h backspace", input: "\x08", wantEv: editBackspace, wantSize: 1},
		{name: "ctrl-a home", input: "\x01", wantEv: editHome, wantSize: 1},
		{name: "ctrl-e end", input: "\x05", wantEv: editEnd, wantSize: 1},
		{name: "ctrl-u clear", input: "\x15", wantEv: editClearLine, wantSize: 1},
		{name: "ascii letter", input: "a", wantEv: editRune, wantRune: 'a', wantSize: 1},
		{name: "umlaut", input: "ä", wantEv: editRune, wantRune: 'ä', wantSize: 2},
		{name: "emoji", input: "\U0001F680", wantEv: editRune, wantRune: '\U0001F680', wantSize: 4},
		{name: "incomplete umlaut waits", input: "\xc3", wantEv: editNone, wantSize: 0},
		{name: "incomplete emoji waits", input: "\xf0\x9f", wantEv: editNone, wantSize: 0},
		{name: "invalid byte skipped", input: "\xff", wantEv: editIgnore, wantSize: 1},
		{name: "lone continuation byte skipped", input: "\x80", wantEv: editIgnore, wantSize: 1},
		{name: "unhandled ctrl byte ignored", input: "\x0b", wantEv: editIgnore, wantSize: 1},
		{name: "bare escape cancels", input: "\x1b", wantEv: editCancel, wantSize: 1},
		{name: "escape before non-CSI cancels", input: "\x1bx", wantEv: editCancel, wantSize: 1},
		{name: "right arrow", input: "\x1b[C", wantEv: editRight, wantSize: 3},
		{name: "left arrow", input: "\x1b[D", wantEv: editLeft, wantSize: 3},
		{name: "csi home", input: "\x1b[H", wantEv: editHome, wantSize: 3},
		{name: "csi end", input: "\x1b[F", wantEv: editEnd, wantSize: 3},
		{name: "tilde home", input: "\x1b[1~", wantEv: editHome, wantSize: 4},
		{name: "tilde end", input: "\x1b[4~", wantEv: editEnd, wantSize: 4},
		{name: "delete key", input: "\x1b[3~", wantEv: editDelete, wantSize: 4},
		{name: "function key consumed whole", input: "\x1b[15~", wantEv: editIgnore, wantSize: 5},
		{name: "bracketed paste on ignored", input: "\x1b[200~", wantEv: editIgnore, wantSize: 6},
		{name: "ss3 right", input: "\x1bOC", wantEv: editRight, wantSize: 3},
		{name: "ss3 left", input: "\x1bOD", wantEv: editLeft, wantSize: 3},
		{name: "ss3 home", input: "\x1bOH", wantEv: editHome, wantSize: 3},
		{name: "ss3 end", input: "\x1bOF", wantEv: editEnd, wantSize: 3},
		{name: "incomplete csi waits", input: "\x1b[", wantEv: editNone, wantSize: 0},
		{name: "incomplete delete waits", input: "\x1b[3", wantEv: editNone, wantSize: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, r, size := decodeEditEvent([]byte(tt.input))
			if ev != tt.wantEv {
				t.Errorf("decodeEditEvent(%q) event = %v, want %v", tt.input, ev, tt.wantEv)
			}
			if ev == editRune && r != tt.wantRune {
				t.Errorf("decodeEditEvent(%q) rune = %q, want %q", tt.input, r, tt.wantRune)
			}
			if size != tt.wantSize {
				t.Errorf("decodeEditEvent(%q) size = %d, want %d", tt.input, size, tt.wantSize)
			}
		})
	}
}

// TestDecodeEditEventReassemblesSplitRunes verifies that multi-byte runes
// split across stdin reads are buffered and decoded instead of dropped,
// mirroring the pending-byte handling in EditLine.
func TestDecodeEditEventReassemblesSplitRunes(t *testing.T) {
	const text = "äö\U0001F680ü" // mix of 2-byte and 4-byte runes

	for split := 1; split < len(text); split++ {
		var pending []byte
		var out []rune

		feed := func(chunk []byte) {
			data := make([]byte, 0, len(pending)+len(chunk))
			data = append(data, pending...)
			data = append(data, chunk...)
			pending = pending[:0]

			i := 0
			for i < len(data) {
				ev, r, size := decodeEditEvent(data[i:])
				if ev == editNone {
					pending = append(pending, data[i:]...)
					return
				}
				i += size
				if ev == editRune {
					out = append(out, r)
				}
			}
		}

		feed([]byte(text[:split]))
		feed([]byte(text[split:]))

		if got := string(out); got != text {
			t.Errorf("split %d: typed %q, want %q", split, got, text)
		}
		if len(pending) != 0 {
			t.Errorf("split %d: %d bytes left pending", split, len(pending))
		}
	}
}

func TestGetEditor(t *testing.T) {
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	origGitVar := gitVarEditor
	defer func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
		gitVarEditor = origGitVar
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

	t.Run("git core.editor before platform default", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")
		gitVarEditor = func() string { return "kate" }
		if got := GetEditor(); got != "kate" {
			t.Errorf("GetEditor() = %q, want \"kate\"", got)
		}
	})

	t.Run("platform default vi", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")
		gitVarEditor = func() string { return "" }
		if runtime.GOOS == "windows" {
			t.Skip("windows default is notepad")
		}
		if got := GetEditor(); got != "vi" {
			t.Errorf("GetEditor() = %q, want \"vi\"", got)
		}
	})
}

// TestEditExternalUsesUniqueTempFile verifies that the external editor gets
// a uniquely named temp file instead of the old predictable fixed path.
func TestEditExternalUsesUniqueTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as fake editor")
	}

	record := filepath.Join(t.TempDir(), "editor-arg.txt")
	editorPath := filepath.Join(t.TempDir(), "fake-editor.sh")
	script := "#!/bin/sh\nprintf '%s' \"$1\" > \"" + record + "\"\nprintf 'edited' > \"$1\"\n"
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VISUAL", editorPath)
	t.Setenv("EDITOR", "")

	got, err := EditExternal("original")
	if err != nil {
		t.Fatalf("EditExternal() error = %v", err)
	}
	if got != "edited" {
		t.Errorf("EditExternal() = %q, want \"edited\"", got)
	}

	raw, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("fake editor did not record its argument: %v", err)
	}
	name := filepath.Base(strings.TrimSpace(string(raw)))
	if name == "yeet-commit-msg.txt" {
		t.Errorf("editor got predictable fixed temp file name %q", name)
	}
	if !strings.HasPrefix(name, "yeet-commit-msg-") || !strings.HasSuffix(name, ".txt") {
		t.Errorf("unexpected temp file name %q", name)
	}
}
