package term

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// EditLine runs an inline editor with cursor movement support.
func EditLine(initial string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return initial, fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.Restore(fd, oldState)

	line := []rune(initial)
	cursor := len(line)
	hasCard := MsgBg != ""
	renderedLines := 0
	linesBelowCursor := 0

	redraw := func() {
		if renderedLines > 0 {
			if linesBelowCursor > 0 {
				fmt.Printf("\033[%dB", linesBelowCursor)
			}
			ClearLines(renderedLines)
		}

		width := TerminalWidth()
		text := string(line)
		if hasCard {
			rows := wrapRunes(text, messageContentWidth(width))
			maxWidth := maxLineWidth(rows)
			pad := strings.Repeat(" ", maxWidth+3)

			fmt.Printf("  %s%s%s", MsgBar, pad, Reset)
			fmt.Print("\n")
			for i, row := range rows {
				rpad := strings.Repeat(" ", maxWidth-len([]rune(row)))
				fmt.Printf("  %s%s%s%s", MsgOpen, row, rpad, MsgClose)
				if i < len(rows)-1 {
					fmt.Print("\n")
				}
			}
			fmt.Print("\n")
			fmt.Printf("  %s%s%s", MsgBar, pad, Reset)

			cursorRow, cursorCol := wrappedCursorPosition(cursor, len(line), messageContentWidth(width))
			renderedLines = len(rows) + 2
			linesBelowCursor = len(rows) - cursorRow
			if linesBelowCursor > 0 {
				fmt.Printf("\033[%dA", linesBelowCursor)
			}
			fmt.Printf("\r\033[%dC", 4+cursorCol)
		} else {
			rows := wrapRunes(text, plainMessageContentWidth(width))
			for i, row := range rows {
				fmt.Printf("  %s%s%s", MsgOpen, row, MsgClose)
				if i < len(rows)-1 {
					fmt.Print("\n")
				}
			}

			cursorRow, cursorCol := wrappedCursorPosition(cursor, len(line), plainMessageContentWidth(width))
			renderedLines = len(rows)
			linesBelowCursor = len(rows) - 1 - cursorRow
			if linesBelowCursor > 0 {
				fmt.Printf("\033[%dA", linesBelowCursor)
			}
			fmt.Printf("\r\033[%dC", 4+cursorCol)
		}
	}

	clearEdit := func() {
		if renderedLines == 0 {
			return
		}
		if linesBelowCursor > 0 {
			fmt.Printf("\033[%dB", linesBelowCursor)
		}
		ClearLines(renderedLines)
		renderedLines = 0
		linesBelowCursor = 0
	}

	redraw()

	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return string(line), err
		}

		for i := 0; i < n; {
			b := buf[i]
			switch {
			case b == 13 || b == 10: // Enter
				clearEdit()
				return string(line), nil
			case b == 27: // Escape sequence
				if i+2 < n && buf[i+1] == '[' {
					switch buf[i+2] {
					case 'C': // Right
						if cursor < len(line) {
							cursor++
						}
					case 'D': // Left
						if cursor > 0 {
							cursor--
						}
					case 'H': // Home
						cursor = 0
					case 'F': // End
						cursor = len(line)
					case '3': // Delete (ESC [ 3 ~)
						if cursor < len(line) {
							line = append(line[:cursor], line[cursor+1:]...)
						}
						if i+3 < n && buf[i+3] == '~' {
							i++
						}
					}
					i += 3
					redraw()
					continue
				}
				// bare Escape — cancel
				clearEdit()
				return initial, nil
			case b == 127 || b == 8: // Backspace
				if cursor > 0 {
					line = append(line[:cursor-1], line[cursor:]...)
					cursor--
				}
			case b == 1: // Ctrl+A — home
				cursor = 0
			case b == 5: // Ctrl+E — end
				cursor = len(line)
			case b == 21: // Ctrl+U — clear line
				line = line[:0]
				cursor = 0
			case b == 3: // Ctrl+C — cancel
				clearEdit()
				return initial, nil
			case b >= 32 && b < 127: // printable ASCII
				line = append(line[:cursor], append([]rune{rune(b)}, line[cursor:]...)...)
				cursor++
			}
			i++
			redraw()
		}
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
