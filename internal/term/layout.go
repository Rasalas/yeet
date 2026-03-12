package term

import (
	"os"

	goterm "golang.org/x/term"
)

const defaultTerminalWidth = 80

func TerminalWidth() int {
	width, _, err := goterm.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return defaultTerminalWidth
	}
	return width
}

func safeTerminalWidth(width int) int {
	if width <= 0 {
		width = defaultTerminalWidth
	}
	if width > 1 {
		return width - 1
	}
	return 1
}

func availableContentWidth(width, chrome int) int {
	width = safeTerminalWidth(width)
	width -= chrome
	if width < 1 {
		return 1
	}
	return width
}

func messageContentWidth(width int) int {
	return availableContentWidth(width, 6)
}

func plainMessageContentWidth(width int) int {
	return availableContentWidth(width, 4)
}

func wrapRunes(text string, width int) []string {
	if width <= 0 {
		width = 1
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return []string{""}
	}

	lines := make([]string, 0, (len(runes)+width-1)/width)
	for start := 0; start < len(runes); start += width {
		end := start + width
		if end > len(runes) {
			end = len(runes)
		}
		lines = append(lines, string(runes[start:end]))
	}
	return lines
}

func wrapMultiline(text string, width int) []string {
	if text == "" {
		return []string{""}
	}

	var lines []string
	for _, line := range splitLines(text) {
		lines = append(lines, wrapRunes(line, width)...)
	}
	return lines
}

func splitLines(text string) []string {
	lines := []string{""}
	start := 0
	for i, r := range text {
		if r != '\n' {
			continue
		}
		lines[len(lines)-1] = text[start:i]
		lines = append(lines, "")
		start = i + 1
	}
	lines[len(lines)-1] = text[start:]
	return lines
}

func maxLineWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		if w := len([]rune(line)); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

func wrappedCursorPosition(cursor, lineLen, width int) (row, col int) {
	if width <= 0 {
		width = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > lineLen {
		cursor = lineLen
	}
	if cursor == lineLen && lineLen > 0 && lineLen%width == 0 {
		return lineLen/width - 1, width
	}
	return cursor / width, cursor % width
}

func MessageCardRows(message string, width int) int {
	return len(wrapRunes(message, messageContentWidth(width)))
}

func PlainMessageRows(message string, width int) int {
	return len(wrapRunes(message, plainMessageContentWidth(width)))
}
