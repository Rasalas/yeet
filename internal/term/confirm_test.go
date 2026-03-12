package term

import "testing"

func TestPrintHintActionsWraps(t *testing.T) {
	width := 20
	actions := []HintAction{
		{Key: "enter", Desc: "commit"},
		{Key: "e", Desc: "edit"},
		{Key: "E", Desc: "editor"},
	}

	if got := PrintHintActions(actions, width); got != 3 {
		t.Fatalf("PrintHintActions() = %d, want %d", got, 3)
	}
}

func TestRenderedBlockClearLinesHelper(t *testing.T) {
	if got := RenderedBlockClearLines(5, 2); got != 8 {
		t.Fatalf("RenderedBlockClearLines() = %d, want %d", got, 8)
	}
}
