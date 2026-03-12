package cmd

import (
	"fmt"

	"github.com/rasalas/yeet/internal/term"
)

type hintAction struct {
	key  string
	desc string
}

func printHintActions(actions []hintAction, width int) int {
	if width <= 0 {
		width = 80
	}
	if width > 1 {
		width--
	}

	const indentWidth = 2
	const separatorWidth = 5

	lines := 1
	lineWidth := indentWidth
	fmt.Print("  ")

	for i, action := range actions {
		segmentWidth := len([]rune(action.key)) + 1 + len([]rune(action.desc))
		if i > 0 && lineWidth+separatorWidth+segmentWidth > width && lineWidth > indentWidth {
			fmt.Print(term.Reset)
			fmt.Print("\n  ")
			lines++
			lineWidth = indentWidth
		} else if i > 0 {
			fmt.Printf("%s  ·  ", term.Dim)
			lineWidth += separatorWidth
		}

		fmt.Print(term.Keyhint(action.key, action.desc))
		lineWidth += segmentWidth
	}

	fmt.Printf("%s\n", term.Reset)
	return lines
}

func clearLinesForRenderedBlocks(renderedLines ...int) int {
	total := 1 // current cursor line below the rendered block
	for _, lines := range renderedLines {
		total += lines
	}
	return total
}

func clearRenderedLines(renderedLines int) {
	term.ClearLines(clearLinesForRenderedBlocks(renderedLines))
}
