package term

import "testing"

func TestMessageAndPlainRows(t *testing.T) {
	if got := MessageCardRows("123456789012345", 20); got != 2 {
		t.Fatalf("MessageCardRows() = %d, want 2", got)
	}

	if got := PlainMessageRows("1234567", 10); got != 2 {
		t.Fatalf("PlainMessageRows() = %d, want 2", got)
	}
}

func TestWrapRunesPrefersWordBoundaries(t *testing.T) {
	got := wrapRunes("clarify usage instructions for wrapping", 18)
	want := []string{"clarify usage", "instructions for", "wrapping"}
	if len(got) != len(want) {
		t.Fatalf("wrapRunes() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrapRunes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWrapRunesHandlesWideUnicode(t *testing.T) {
	got := wrapRunes("fix 漢字 support", 10)
	want := []string{"fix 漢字", "support"}
	if len(got) != len(want) {
		t.Fatalf("wrapRunes() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("wrapRunes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVisibleWindowUsesDisplayWidth(t *testing.T) {
	line := []rune("fix 漢字 support")
	cursor := len(line)
	start, end, cursorCol := visibleWindow(line, cursor, 8)

	if got := string(line[start:end]); got != "support" {
		t.Fatalf("visibleWindow() text = %q, want %q", got, "support")
	}
	if cursorCol != 7 {
		t.Fatalf("visibleWindow() cursorCol = %d, want 7", cursorCol)
	}
}

func TestWrappedCursorPosition(t *testing.T) {
	tests := []struct {
		name    string
		cursor  int
		lineLen int
		width   int
		wantRow int
		wantCol int
	}{
		{name: "middle of first row", cursor: 3, lineLen: 8, width: 6, wantRow: 0, wantCol: 3},
		{name: "start of wrapped row", cursor: 6, lineLen: 8, width: 6, wantRow: 1, wantCol: 0},
		{name: "exact end stays on last row", cursor: 6, lineLen: 6, width: 6, wantRow: 0, wantCol: 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col := wrappedCursorPosition(tt.cursor, tt.lineLen, tt.width)
			if row != tt.wantRow || col != tt.wantCol {
				t.Fatalf("wrappedCursorPosition(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tt.cursor, tt.lineLen, tt.width, row, col, tt.wantRow, tt.wantCol)
			}
		})
	}
}
