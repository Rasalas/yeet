package git

import (
	"fmt"
	"testing"
)

// mockGit implements the Git interface for testing.
type mockGit struct {
	hasStagedChanges bool
	stageAllErr      error
	diffStat         string
	diffStatErr      error
	diffCached       string
	diffCachedErr    error
	commitOut        string
	commitErr        error
	pushOut          string
	pushErr          error
	pushSetUp        string
	pushSetUpErr     error
	resetErr         error
	logOneline       string
	logOnelineErr    error
	statusShort      string
	statusShortErr   error
	currentBranch    string
	currentBranchErr error
	defaultBranch    string
	defaultBranchErr error
	logRange         string
	logRangeErr      error
	diffRange        string
	diffRangeErr     error
	diffStatRange    string
	diffStatRangeErr error
	hasUpstream      bool
}

func (m mockGit) HasStagedChanges() bool                     { return m.hasStagedChanges }
func (m mockGit) StageAll() error                            { return m.stageAllErr }
func (m mockGit) DiffStat() (string, error)                  { return m.diffStat, m.diffStatErr }
func (m mockGit) DiffCached() (string, error)                { return m.diffCached, m.diffCachedErr }
func (m mockGit) Commit(msg string) (string, error)          { return m.commitOut, m.commitErr }
func (m mockGit) Push() (string, error)                      { return m.pushOut, m.pushErr }
func (m mockGit) PushSetUpstream() (string, error)           { return m.pushSetUp, m.pushSetUpErr }
func (m mockGit) Reset() error                               { return m.resetErr }
func (m mockGit) LogOneline() (string, error)                { return m.logOneline, m.logOnelineErr }
func (m mockGit) StatusShort() (string, error)               { return m.statusShort, m.statusShortErr }
func (m mockGit) CurrentBranch() (string, error)             { return m.currentBranch, m.currentBranchErr }
func (m mockGit) DefaultBranch() (string, error)             { return m.defaultBranch, m.defaultBranchErr }
func (m mockGit) LogRange(base string) (string, error)       { return m.logRange, m.logRangeErr }
func (m mockGit) DiffRange(base string) (string, error)      { return m.diffRange, m.diffRangeErr }
func (m mockGit) DiffStatRange(base string) (string, error)  { return m.diffStatRange, m.diffStatRangeErr }
func (m mockGit) HasUpstream() bool                          { return m.hasUpstream }

func TestFreeFunctionsDelegateToDefault(t *testing.T) {
	original := Default
	defer func() { Default = original }()

	mock := mockGit{
		hasStagedChanges: true,
		currentBranch:    "feat/test",
		logOneline:       "abc123 some commit",
		statusShort:      "M foo.go",
		defaultBranch:    "main",
		logRange:         "abc123 commit one\ndef456 commit two",
		diffRange:        "diff --git a/foo.go b/foo.go",
		diffStatRange:    " foo.go | 3 +++",
		hasUpstream:      true,
	}
	Default = mock

	if !HasStagedChanges() {
		t.Error("HasStagedChanges: expected true")
	}

	branch, err := CurrentBranch()
	if err != nil || branch != "feat/test" {
		t.Errorf("CurrentBranch = %q, %v", branch, err)
	}

	log, err := LogOneline()
	if err != nil || log != "abc123 some commit" {
		t.Errorf("LogOneline = %q, %v", log, err)
	}

	status, err := StatusShort()
	if err != nil || status != "M foo.go" {
		t.Errorf("StatusShort = %q, %v", status, err)
	}

	defBranch, err := DefaultBranch()
	if err != nil || defBranch != "main" {
		t.Errorf("DefaultBranch = %q, %v", defBranch, err)
	}

	lr, err := LogRange("main")
	if err != nil || lr != "abc123 commit one\ndef456 commit two" {
		t.Errorf("LogRange = %q, %v", lr, err)
	}

	dr, err := DiffRange("main")
	if err != nil || dr != "diff --git a/foo.go b/foo.go" {
		t.Errorf("DiffRange = %q, %v", dr, err)
	}

	dsr, err := DiffStatRange("main")
	if err != nil || dsr != " foo.go | 3 +++" {
		t.Errorf("DiffStatRange = %q, %v", dsr, err)
	}

	if !HasUpstream() {
		t.Error("HasUpstream: expected true")
	}
}

func TestFreeFunctionsErrorDelegation(t *testing.T) {
	original := Default
	defer func() { Default = original }()

	mock := mockGit{
		stageAllErr:      fmt.Errorf("stage error"),
		currentBranchErr: fmt.Errorf("branch error"),
		defaultBranchErr: fmt.Errorf("default branch error"),
	}
	Default = mock

	if err := StageAll(); err == nil {
		t.Error("StageAll: expected error")
	}

	_, err := CurrentBranch()
	if err == nil {
		t.Error("CurrentBranch: expected error")
	}

	_, err = DefaultBranch()
	if err == nil {
		t.Error("DefaultBranch: expected error")
	}
}
