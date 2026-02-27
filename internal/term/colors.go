package term

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Hex color constants — single source of truth for the entire app.
const (
	HexPrimary   = "#FF8C42"
	HexSecondary = "#FFB074"
	HexMuted     = "#78716C"
	HexSuccess   = "#4ADE80"
	HexDanger    = "#FF6B6B"
	HexWarning   = "#FBBF24"
	HexText      = "#FEF9F3"
)

// ANSI color codes — disabled when NO_COLOR is set.
var (
	Bold  = "\033[1m"
	Dim   = "\033[2m"
	Red   = "\033[31m"
	Green = "\033[32m"
	Reset = "\033[0m"

	// Commit message display: colored left bar on warm dark background.
	MsgBar   = "\033[38;2;255;140;66m\033[48;2;44;36;30m\u258e\033[39m"
	MsgBg    = "\033[48;2;44;36;30m"
	MsgOpen  = "\033[38;2;255;140;66m\033[48;2;44;36;30m\u258e\033[39m\033[1m "
	MsgClose = "  \033[0m"
	MsgPad   = 2
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		Bold = ""
		Dim = ""
		Red = ""
		Green = ""
		Reset = ""
		MsgBar = ""
		MsgBg = ""
		MsgOpen = "\u203a "
		MsgClose = ""
		MsgPad = 0
	}
}

// Keyhint formats a keybinding: key in bold, description in dim.
func Keyhint(key, desc string) string {
	return Reset + Bold + key + Reset + Dim + " " + desc
}

// ClearLine clears the current line and returns the cursor to column 0.
func ClearLine() {
	fmt.Print("\r\033[K")
}

// ClearLines clears n lines going upward from the current cursor position.
func ClearLines(n int) {
	fmt.Print("\033[2K")
	for i := 1; i < n; i++ {
		fmt.Print("\033[1A\033[2K")
	}
	fmt.Print("\r")
}

// diffStatRe matches lines like "  file.go | 29 ++---"
var diffStatRe = regexp.MustCompile(`^(.*\|[^+-]*?)(\+*)(-*)$`)

// ColorizeDiffStat colorizes a single diff stat line.
func ColorizeDiffStat(line string) string {
	if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
		return Dim + line + Reset
	}

	if m := diffStatRe.FindStringSubmatch(line); m != nil {
		prefix := m[1]
		plus := m[2]
		minus := m[3]
		var b strings.Builder
		b.WriteString(prefix)
		if plus != "" {
			b.WriteString(Green + plus + Reset)
		}
		if minus != "" {
			b.WriteString(Red + minus + Reset)
		}
		return b.String()
	}

	return line
}
