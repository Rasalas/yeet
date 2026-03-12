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
