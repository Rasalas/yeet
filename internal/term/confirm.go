package term

import "fmt"

type HintAction struct {
	Key  string
	Desc string
}

func PrintHintActions(actions []HintAction, width int) int {
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
		segmentWidth := displayWidth(action.Key) + 1 + displayWidth(action.Desc)
		if i > 0 && lineWidth+separatorWidth+segmentWidth > width && lineWidth > indentWidth {
			fmt.Print(Reset)
			fmt.Print("\n  ")
			lines++
			lineWidth = indentWidth
		} else if i > 0 {
			fmt.Printf("%s  ·  ", Dim)
			lineWidth += separatorWidth
		}

		fmt.Print(Keyhint(action.Key, action.Desc))
		lineWidth += segmentWidth
	}

	fmt.Printf("%s\n", Reset)
	return lines
}

func RenderedBlockClearLines(renderedLines ...int) int {
	total := 1 // current cursor line below the rendered block
	for _, lines := range renderedLines {
		total += lines
	}
	return total
}

func ClearRenderedBlock(renderedLines ...int) {
	ClearLines(RenderedBlockClearLines(renderedLines...))
}
