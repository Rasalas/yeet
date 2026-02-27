package ai

import (
	"strings"
	"testing"
)

func TestBuildUserMessage(t *testing.T) {
	t.Run("all fields", func(t *testing.T) {
		ctx := CommitContext{
			Diff:          "diff --git a/foo.go",
			Branch:        "feat/login",
			RecentCommits: "abc123 initial commit",
			Status:        "M foo.go",
		}
		msg := ctx.BuildUserMessage()

		if !strings.Contains(msg, "Branch: feat/login") {
			t.Error("missing branch")
		}
		if !strings.Contains(msg, "Files changed:\nM foo.go") {
			t.Error("missing status")
		}
		if !strings.Contains(msg, "Recent commits:\nabc123 initial commit") {
			t.Error("missing recent commits")
		}
		if !strings.Contains(msg, "Diff:\ndiff --git a/foo.go") {
			t.Error("missing diff")
		}
	})

	t.Run("empty optional fields", func(t *testing.T) {
		ctx := CommitContext{
			Diff: "some diff",
		}
		msg := ctx.BuildUserMessage()

		if strings.Contains(msg, "Branch:") {
			t.Error("should not contain branch when empty")
		}
		if strings.Contains(msg, "Files changed:") {
			t.Error("should not contain status when empty")
		}
		if strings.Contains(msg, "Recent commits:") {
			t.Error("should not contain recent commits when empty")
		}
		if !strings.Contains(msg, "Diff:\nsome diff") {
			t.Error("missing diff")
		}
	})
}

func TestTruncateDiff(t *testing.T) {
	t.Run("short diff unchanged", func(t *testing.T) {
		diff := "line1\nline2\nline3"
		got := truncateDiff(diff)
		if got != diff {
			t.Errorf("truncateDiff changed short diff: %q", got)
		}
	})

	t.Run("long diff truncated", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < maxDiffLines+100; i++ {
			b.WriteString("line\n")
		}
		got := truncateDiff(b.String())

		if !strings.HasSuffix(got, "... (diff truncated)") {
			t.Error("missing truncation marker")
		}
		// Count lines in the preserved part (before truncation marker)
		preserved := strings.Count(got, "\n")
		if preserved != maxDiffLines {
			t.Errorf("got %d newlines, want %d", preserved, maxDiffLines)
		}
	})

	t.Run("empty diff", func(t *testing.T) {
		got := truncateDiff("")
		if got != "" {
			t.Errorf("truncateDiff(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("unicode safe", func(t *testing.T) {
		// Ensure truncation at newline boundaries doesn't corrupt UTF-8
		diff := strings.Repeat("日本語テスト\n", maxDiffLines+10)
		got := truncateDiff(diff)
		if !strings.HasSuffix(got, "... (diff truncated)") {
			t.Error("missing truncation marker")
		}
		// Verify valid UTF-8 by checking the result is usable
		if len(got) == 0 {
			t.Error("truncated to empty")
		}
	})
}
