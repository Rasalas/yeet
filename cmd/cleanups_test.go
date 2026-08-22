package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rasalas/yeet/internal/git"
)

func TestRunYeetCommitFailureSurfacesGitOutput(t *testing.T) {
	origGit := git.Default
	origMessageFlag := messageFlag
	origYesFlag := yesFlag
	defer func() {
		git.Default = origGit
		messageFlag = origMessageFlag
		yesFlag = origYesFlag
	}()

	git.Default = &runYeetMockGit{
		hasStagedChanges: true,
		diffStat:         " cmd/root.go | 3 ++-",
		commitOut:        "error: pre-commit hook declined",
		commitErr:        errors.New("exit status 1"),
		currentBranch:    "main",
	}
	messageFlag = "test message"
	yesFlag = true

	err := runYeet(rootCmd, nil)
	if err == nil {
		t.Fatal("expected commit failure to surface as error")
	}
	if !strings.Contains(err.Error(), "pre-commit hook declined") {
		t.Errorf("error = %q, want git output included", err.Error())
	}
}

func TestRunYeetCommitFailureWrapsWhenOutputEmpty(t *testing.T) {
	origGit := git.Default
	origMessageFlag := messageFlag
	origYesFlag := yesFlag
	defer func() {
		git.Default = origGit
		messageFlag = origMessageFlag
		yesFlag = origYesFlag
	}()

	git.Default = &runYeetMockGit{
		hasStagedChanges: true,
		diffStat:         " cmd/root.go | 3 ++-",
		commitErr:        errors.New("exit status 128"),
		currentBranch:    "main",
	}
	messageFlag = "test message"
	yesFlag = true

	err := runYeet(rootCmd, nil)
	if err == nil {
		t.Fatal("expected commit failure to surface as error")
	}
	if !strings.Contains(err.Error(), "commit failed") {
		t.Errorf("error = %q, want 'commit failed' context", err.Error())
	}
}

func TestFileDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if got := fileDigest(path); got != "" {
		t.Errorf("fileDigest(missing) = %q, want empty", got)
	}

	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := fileDigest(path)
	if first == "" {
		t.Fatal("fileDigest returned empty for existing file")
	}

	if err := os.WriteFile(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if second := fileDigest(path); second == first {
		t.Error("digest did not change after content change")
	}
}
