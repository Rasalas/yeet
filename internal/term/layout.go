package term

import (
	"os"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
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

	if text == "" {
		return []string{""}
	}

	runes := []rune(text)
	var lines []string
	for start := 0; start < len(runes); {
		end, next := wrappedLineBreak(runes, start, width)
		lines = append(lines, strings.TrimRightFunc(string(runes[start:end]), unicode.IsSpace))
		start = skipLeadingSpaces(runes, next)
		if start >= len(runes) && next < len(runes) {
			lines = append(lines, "")
		}
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
		if w := displayWidth(line); w > maxWidth {
			maxWidth = w
		}
	}
	return maxWidth
}

func displayWidth(text string) int {
	return runewidth.StringWidth(text)
}

func displayWidthRunes(runes []rune) int {
	width := 0
	for _, r := range runes {
		width += runeDisplayWidth(r)
	}
	return width
}

func runeDisplayWidth(r rune) int {
	if r == '\t' {
		return 4
	}
	if w := runewidth.RuneWidth(r); w > 0 {
		return w
	}
	return 0
}

func padCells(cells int) string {
	if cells <= 0 {
		return ""
	}
	return strings.Repeat(" ", cells)
}

func wrappedLineBreak(runes []rune, start, width int) (end, next int) {
	lineWidth := 0
	lastBreak := -1

	for i := start; i < len(runes); i++ {
		rw := runeDisplayWidth(runes[i])
		if lineWidth > 0 && lineWidth+rw > width {
			if lastBreak >= start {
				return lastBreak + 1, lastBreak + 1
			}
			return i, i
		}
		if lineWidth == 0 && rw > width {
			return i + 1, i + 1
		}

		lineWidth += rw
		if unicode.IsSpace(runes[i]) {
			lastBreak = i
		}
	}

	return len(runes), len(runes)
}

func skipLeadingSpaces(runes []rune, start int) int {
	for start < len(runes) && unicode.IsSpace(runes[start]) {
		start++
	}
	return start
}

func visibleWindow(runes []rune, cursor, width int) (start, end, cursorCol int) {
	if width <= 0 {
		width = 1
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	start = cursor
	used := 0
	for start > 0 {
		rw := runeDisplayWidth(runes[start-1])
		if used+rw > width {
			break
		}
		used += rw
		start--
	}

	end = cursor
	used = displayWidthRunes(runes[start:cursor])
	for end < len(runes) {
		rw := runeDisplayWidth(runes[end])
		if used+rw > width {
			break
		}
		used += rw
		end++
	}

	trimmedStart := skipLeadingSpaces(runes, start)
	if trimmedStart < cursor {
		start = trimmedStart
	}
	for end < len(runes) {
		used = displayWidthRunes(runes[start:end])
		rw := runeDisplayWidth(runes[end])
		if used+rw > width {
			break
		}
		end++
	}

	cursorCol = displayWidthRunes(runes[start:cursor])
	return start, end, cursorCol
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
