package term

import (
	"fmt"
	"strings"
)

// DisplayCard renders a title+body card in the same style as PR/commit previews.
// Returns the number of terminal lines used (for ClearLines).
func DisplayCard(title, body string) int {
	lines := 0
	width := TerminalWidth()
	titleLines := wrapRunes("# "+title, messageContentWidth(width))
	bodyLines := []string(nil)
	if body != "" {
		bodyLines = wrapMultiline(body, messageContentWidth(width))
	}

	content := append([]string{}, titleLines...)
	if body != "" {
		content = append(content, "")
		content = append(content, bodyLines...)
	}
	maxWidth := maxLineWidth(content)

	if MsgBg != "" {
		pad := strings.Repeat(" ", maxWidth+3)
		fmt.Printf("  %s%s%s\n", MsgBar, pad, Reset)
		lines++
		for i, line := range content {
			rpad := strings.Repeat(" ", maxWidth-len([]rune(line)))
			if i < len(titleLines) {
				fmt.Printf("  %s%s%s%s\n", MsgOpen, line, rpad, MsgClose)
			} else {
				fmt.Printf("  %s%s %s%s  %s\n", MsgBar, Dim, line, rpad, Reset)
			}
			lines++
		}
		fmt.Printf("  %s%s%s\n", MsgBar, pad, Reset)
		lines++
	} else {
		for i, line := range content {
			if i < len(titleLines) {
				fmt.Printf("  %s%s%s\n", MsgOpen, line, MsgClose)
			} else {
				fmt.Printf("  %s\n", line)
			}
			lines++
		}
	}

	fmt.Println()
	lines++
	return lines
}

// DisplayMessage renders a commit message preview within the current terminal width.
// Returns the number of terminal lines used, including the blank line after the preview.
func DisplayMessage(message string, width int) int {
	if MsgBg != "" {
		rows := wrapRunes(message, messageContentWidth(width))
		maxWidth := maxLineWidth(rows)
		pad := strings.Repeat(" ", maxWidth+3)

		fmt.Printf("  %s%s%s\n", MsgBar, pad, Reset)
		for _, row := range rows {
			rpad := strings.Repeat(" ", maxWidth-len([]rune(row)))
			fmt.Printf("  %s%s%s%s\n", MsgOpen, row, rpad, MsgClose)
		}
		fmt.Printf("  %s%s%s\n\n", MsgBar, pad, Reset)
		return len(rows) + 3
	}

	rows := wrapRunes(message, plainMessageContentWidth(width))
	for _, row := range rows {
		fmt.Printf("  %s%s%s\n", MsgOpen, row, MsgClose)
	}
	fmt.Println()
	return len(rows) + 1
}
