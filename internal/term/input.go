package term

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// Action represents a user action from the confirmation prompt.
type Action int

const (
	ActionConfirm Action = iota
	ActionCancel
	ActionEdit
	ActionEditExternal
)

// WaitForAction waits for the user to press a key and returns the corresponding action.
func WaitForAction() (Action, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return ActionCancel, fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return ActionCancel, err
		}
		for i := 0; i < n; i++ {
			switch buf[i] {
			case 13, 10: // Enter
				return ActionConfirm, nil
			case 27: // Escape
				return ActionCancel, nil
			case 3: // Ctrl+C
				return ActionCancel, nil
			case 'q':
				return ActionCancel, nil
			case 'e':
				return ActionEdit, nil
			case 'E':
				return ActionEditExternal, nil
			}
		}
	}
}

// WaitForYesNo waits for the user to press y/n or Enter/Esc.
func WaitForYesNo() (bool, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, err
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			return false, err
		}
		switch buf[0] {
		case 'y', 'Y', 13, 10: // y or Enter
			return true, nil
		case 'n', 'N', 27, 3: // n, Esc, Ctrl+C
			return false, nil
		}
	}
}

// editEvent identifies a decoded key press for the inline editor.
type editEvent int

const (
	editNone      editEvent = iota // incomplete input — caller must read more bytes
	editRune                       // printable rune (see decodeEditEvent's rune result)
	editBackspace
	editDelete
	editLeft
	editRight
	editHome
	editEnd
	editClearLine // Ctrl+U
	editEnter
	editCancel // bare Escape — discard edits
	editAbort  // Ctrl+C — discard edits
	editIgnore // recognized control byte without editor meaning
)

// decodeEditEvent decodes the next key event from the start of data.
//
// It returns the event kind, its rune (for editRune), and the number of
// bytes consumed. Multi-byte UTF-8 runes and escape sequences that are
// split across stdin reads yield editNone with consumed 0 so the caller
// can wait for more bytes instead of dropping input.
func decodeEditEvent(data []byte) (editEvent, rune, int) {
	if len(data) == 0 {
		return editNone, 0, 0
	}
	b := data[0]
	switch {
	case b == 13 || b == 10:
		return editEnter, 0, 1
	case b == 3: // Ctrl+C
		return editAbort, 0, 1
	case b == 127 || b == 8:
		return editBackspace, 0, 1
	case b == 1: // Ctrl+A — home
		return editHome, 0, 1
	case b == 5: // Ctrl+E — end
		return editEnd, 0, 1
	case b == 21: // Ctrl+U — clear line
		return editClearLine, 0, 1
	case b == 27:
		return decodeEscapeEvent(data)
	case b < 32:
		return editIgnore, 0, 1
	default:
		if !utf8.FullRune(data) && b >= utf8.RuneSelf {
			// Possibly an incomplete multi-byte rune at the end of the chunk.
			return editNone, 0, 0
		}
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 {
			// Invalid encoding — skip it rather than stalling the editor.
			return editIgnore, 0, size
		}
		return editRune, r, size
	}
}

// decodeEscapeEvent handles ESC-prefixed keys. A bare or unknown ESC cancels
// editing (discarding changes), matching long-standing yeet behavior.
func decodeEscapeEvent(data []byte) (editEvent, rune, int) {
	if len(data) == 1 {
		return editCancel, 0, 1
	}
	switch data[1] {
	case '[': // CSI sequences
		if len(data) < 3 {
			return editNone, 0, 0
		}
		switch data[2] {
		case 'C':
			return editRight, 0, 3
		case 'D':
			return editLeft, 0, 3
		case 'H':
			return editHome, 0, 3
		case 'F':
			return editEnd, 0, 3
		default:
			n := csiSequenceLength(data)
			if n == 0 {
				return editNone, 0, 0
			}
			if n == 4 && data[3] == '~' { // single-param tilde keys only
				switch data[2] {
				case '1':
					return editHome, 0, n // ESC[1~
				case '4':
					return editEnd, 0, n // ESC[4~
				case '3':
					return editDelete, 0, n // ESC[3~
				}
			}
			return editIgnore, 0, n
		}
	case 'O': // SS3 sequences (xterm application cursor keys)
		if len(data) < 3 {
			return editNone, 0, 0
		}
		switch data[2] {
		case 'C':
			return editRight, 0, 3
		case 'D':
			return editLeft, 0, 3
		case 'H':
			return editHome, 0, 3
		case 'F':
			return editEnd, 0, 3
		default:
			return editIgnore, 0, 3
		}
	default:
		return editCancel, 0, 1
	}
}

// csiSequenceLength returns the total length of the CSI sequence beginning
// with ESC[ at data[0..1], or 0 if the sequence is incomplete.
func csiSequenceLength(data []byte) int {
	for i := 2; i < len(data); i++ {
		if data[i] >= 0x40 && data[i] <= 0x7E { // final byte terminates
			return i + 1
		}
	}
	return 0
}

// EditLine runs an inline editor with cursor movement support.
//
// Keys: arrows/Home/End move the cursor, Backspace/Delete edit text,
// Ctrl+A/Ctrl+E jump to start/end, Ctrl+U clears the line. Enter accepts
// the line; Escape and Ctrl+C discard all edits and return the initial text.
func EditLine(initial string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return initial, fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.Restore(fd, oldState)

	line := []rune(initial)
	cursor := len(line)
	rendered := false

	redraw := func() {
		if rendered {
			fmt.Print("\r")
			ClearLines(1)
		}

		viewWidth := availableContentWidth(TerminalWidth(), 0)
		start, end, cursorCol := visibleWindow(line, cursor, viewWidth)
		visible := string(line[start:end])

		fmt.Printf("%s%s%s", Primary, visible, Reset)
		fmt.Printf("\r\033[%dC", cursorCol)
		rendered = true
	}

	clearEdit := func() {
		if !rendered {
			return
		}
		fmt.Print("\r")
		ClearLines(1)
		rendered = false
	}

	redraw()

	var pending []byte
	buf := make([]byte, 256)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return string(line), err
		}

		data := make([]byte, 0, len(pending)+n)
		data = append(data, pending...)
		data = append(data, buf[:n]...)

		i := 0
		for i < len(data) {
			ev, r, size := decodeEditEvent(data[i:])
			if ev == editNone {
				break
			}
			i += size

			switch ev {
			case editEnter:
				clearEdit()
				return string(line), nil
			case editAbort, editCancel:
				clearEdit()
				return initial, nil
			case editRune:
				line = append(line[:cursor], append([]rune{r}, line[cursor:]...)...)
				cursor++
			case editBackspace:
				if cursor > 0 {
					line = append(line[:cursor-1], line[cursor:]...)
					cursor--
				}
			case editDelete:
				if cursor < len(line) {
					line = append(line[:cursor], line[cursor+1:]...)
				}
			case editLeft:
				if cursor > 0 {
					cursor--
				}
			case editRight:
				if cursor < len(line) {
					cursor++
				}
			case editHome:
				cursor = 0
			case editEnd:
				cursor = len(line)
			case editClearLine:
				line = line[:0]
				cursor = 0
			}
			redraw()
		}

		pending = append(pending[:0], data[i:]...)
	}
}

// GetEditor returns the user's preferred editor.
func GetEditor() string {
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

// EditExternal opens the message in an external editor.
func EditExternal(initial string) (string, error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "yeet-commit-msg.txt")
	if err := os.WriteFile(tmpFile, []byte(initial), 0600); err != nil {
		return initial, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	editor := GetEditor()
	cmd := exec.Command(editor, tmpFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return initial, fmt.Errorf("editor exited with error: %w", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		return initial, fmt.Errorf("failed to read edited file: %w", err)
	}

	edited := strings.TrimSpace(string(content))
	if edited == "" {
		return initial, nil
	}
	return edited, nil
}
