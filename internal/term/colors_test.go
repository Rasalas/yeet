package term

import (
	"strings"
	"testing"
)

func TestColorizeDiffStat(t *testing.T) {
	t.Run("file with insertions and deletions", func(t *testing.T) {
		line := " cmd/root.go | 29 +++---"
		got := ColorizeDiffStat(line)

		if !strings.Contains(got, Green+"+++"+Reset) {
			t.Errorf("missing green insertions in %q", got)
		}
		if !strings.Contains(got, Red+"---"+Reset) {
			t.Errorf("missing red deletions in %q", got)
		}
	})

	t.Run("file with insertions only", func(t *testing.T) {
		line := " stream.go | 45 +++++++++++++"
		got := ColorizeDiffStat(line)

		if !strings.Contains(got, Green) {
			t.Error("missing green for insertions")
		}
		if strings.Contains(got, Red) {
			t.Error("should not contain red for insertions-only line")
		}
	})

	t.Run("summary line", func(t *testing.T) {
		line := " 5 files changed, 428 insertions(+), 594 deletions(-)"
		got := ColorizeDiffStat(line)

		if !strings.HasPrefix(got, Dim) {
			t.Errorf("summary line should start with dim: %q", got)
		}
		if !strings.HasSuffix(got, Reset) {
			t.Errorf("summary line should end with reset: %q", got)
		}
	})

	t.Run("single file changed", func(t *testing.T) {
		line := " 1 file changed, 3 insertions(+)"
		got := ColorizeDiffStat(line)

		if !strings.HasPrefix(got, Dim) {
			t.Error("single file summary should be dim")
		}
	})

	t.Run("unrecognized line passthrough", func(t *testing.T) {
		line := "some random text"
		got := ColorizeDiffStat(line)
		if got != line {
			t.Errorf("unrecognized line should pass through unchanged, got %q", got)
		}
	})
}

func TestColorizeDiffStatNoColor(t *testing.T) {
	origBold, origDim, origRed, origGreen, origReset := Bold, Dim, Red, Green, Reset
	Bold, Dim, Red, Green, Reset = "", "", "", "", ""
	defer func() { Bold, Dim, Red, Green, Reset = origBold, origDim, origRed, origGreen, origReset }()

	line := " cmd/root.go | 29 +++---"
	got := ColorizeDiffStat(line)

	if strings.Contains(got, "\033") {
		t.Errorf("NO_COLOR output contains escape codes: %q", got)
	}
}
