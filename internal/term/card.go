package term

import (
	"fmt"
	"strings"
)

// DisplayCard renders a title+body card in the same style as PR/commit previews.
// Returns the number of terminal lines used (for ClearLines).
func DisplayCard(title, body string) int {
	lines := 0

	var content []string
	content = append(content, "# "+title)
	if body != "" {
		content = append(content, "")
		content = append(content, strings.Split(body, "\n")...)
	}

	maxWidth := 0
	for _, line := range content {
		if w := len([]rune(line)); w > maxWidth {
			maxWidth = w
		}
	}

	if MsgBg != "" {
		pad := strings.Repeat(" ", maxWidth+3)
		fmt.Printf("  %s%s%s\n", MsgBar, pad, Reset)
		lines++
		for i, line := range content {
			rpad := strings.Repeat(" ", maxWidth-len([]rune(line)))
			if i == 0 {
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
			if i == 0 {
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
