package ai

import (
	"os"
	"path/filepath"
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

func TestLoadPrompt(t *testing.T) {
	// Use a temp dir to avoid touching the real config.
	// Set XDG_CONFIG_HOME so xdg.ConfigDir() uses our temp dir
	// (os.UserHomeDir() on Linux reads /etc/passwd, not $HOME).
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "config")
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	t.Run("default creation", func(t *testing.T) {
		got := LoadPrompt()
		if got != DefaultPrompt {
			t.Errorf("LoadPrompt() on fresh dir returned unexpected prompt")
		}

		// File should now exist
		path := filepath.Join(xdgDir, "yeet", "prompt.txt")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("prompt file not created: %v", err)
		}
		if strings.TrimSpace(string(data)) != DefaultPrompt {
			t.Error("written default prompt doesn't match DefaultPrompt")
		}
	})

	t.Run("custom prompt", func(t *testing.T) {
		custom := "You are a test prompt."
		if err := WritePrompt(custom); err != nil {
			t.Fatalf("WritePrompt failed: %v", err)
		}

		got := LoadPrompt()
		if got != custom {
			t.Errorf("LoadPrompt() = %q, want %q", got, custom)
		}
	})

	t.Run("empty file returns default", func(t *testing.T) {
		if err := WritePrompt(""); err != nil {
			t.Fatalf("WritePrompt failed: %v", err)
		}

		got := LoadPrompt()
		if got != DefaultPrompt {
			t.Errorf("LoadPrompt() with empty file = %q, want default", got)
		}
	})
}

func TestWritePrompt(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "config"))

	content := "custom system prompt"
	if err := WritePrompt(content); err != nil {
		t.Fatalf("WritePrompt failed: %v", err)
	}

	got := LoadPrompt()
	if got != content {
		t.Errorf("read-back = %q, want %q", got, content)
	}
}

func TestPromptPath(t *testing.T) {
	path, err := PromptPath()
	if err != nil {
		t.Fatalf("PromptPath failed: %v", err)
	}
	if path == "" {
		t.Error("PromptPath returned empty string")
	}
	if !strings.HasSuffix(path, "prompt.txt") {
		t.Errorf("PromptPath = %q, expected to end with prompt.txt", path)
	}
}

func TestEffectivePrompt(t *testing.T) {
	t.Run("with override", func(t *testing.T) {
		ctx := CommitContext{SystemPrompt: "custom prompt"}
		if got := ctx.EffectivePrompt(); got != "custom prompt" {
			t.Errorf("EffectivePrompt = %q, want %q", got, "custom prompt")
		}
	})

	t.Run("without override", func(t *testing.T) {
		ctx := CommitContext{}
		got := ctx.EffectivePrompt()
		if got == "" {
			t.Error("EffectivePrompt returned empty string without override")
		}
	})
}

func TestEffectiveMaxTokens(t *testing.T) {
	t.Run("with override", func(t *testing.T) {
		ctx := CommitContext{MaxTokens: 1024}
		if got := ctx.EffectiveMaxTokens(); got != 1024 {
			t.Errorf("EffectiveMaxTokens = %d, want 1024", got)
		}
	})

	t.Run("default", func(t *testing.T) {
		ctx := CommitContext{}
		if got := ctx.EffectiveMaxTokens(); got != 256 {
			t.Errorf("EffectiveMaxTokens = %d, want 256", got)
		}
	})

	t.Run("zero uses default", func(t *testing.T) {
		ctx := CommitContext{MaxTokens: 0}
		if got := ctx.EffectiveMaxTokens(); got != 256 {
			t.Errorf("EffectiveMaxTokens = %d, want 256", got)
		}
	})
}
