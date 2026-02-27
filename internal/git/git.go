package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// Git defines the interface for git operations, enabling test doubles.
type Git interface {
	HasStagedChanges() bool
	StageAll() error
	DiffStat() (string, error)
	DiffCached() (string, error)
	Commit(message string) (string, error)
	Push() (string, error)
	PushSetUpstream() (string, error)
	Reset() error
	Log() (string, error)
	LogOneline() (string, error)
	StatusShort() (string, error)
	CurrentBranch() (string, error)
	DefaultBranch() (string, error)
	LogRange(base string) (string, error)
	DiffRange(base string) (string, error)
	DiffStatRange(base string) (string, error)
	HasUpstream() bool
}

// Default is the package-level Git implementation used by free functions.
// Tests can replace this with a mock.
var Default Git = ExecGit{}

// ExecGit implements Git by shelling out to the git CLI.
type ExecGit struct{}

func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (ExecGit) StageAll() error {
	_, err := run("add", "--all")
	return err
}

func (ExecGit) DiffStat() (string, error) {
	return run("diff", "--cached", "--stat")
}

func (ExecGit) DiffCached() (string, error) {
	return run("diff", "--cached")
}

func (ExecGit) Commit(message string) (string, error) {
	return run("commit", "-m", message)
}

func (ExecGit) Push() (string, error) {
	return run("push")
}

func (ExecGit) Reset() error {
	_, err := run("reset")
	return err
}

func (ExecGit) Log() (string, error) {
	return run("log", "--oneline", "--graph", "--decorate", "-20")
}

func (ExecGit) LogOneline() (string, error) {
	return run("log", "--oneline", "-10")
}

func (ExecGit) HasStagedChanges() bool {
	out, err := run("diff", "--cached", "--quiet")
	return err != nil && out == ""
}

func (ExecGit) StatusShort() (string, error) {
	return run("status", "--short")
}

func (ExecGit) CurrentBranch() (string, error) {
	return run("rev-parse", "--abbrev-ref", "HEAD")
}

func (ExecGit) PushSetUpstream() (string, error) {
	branch, err := run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return run("push", "--set-upstream", "origin", branch)
}

// DefaultBranch detects the default branch (main/master) of the repository.
func (ExecGit) DefaultBranch() (string, error) {
	// Try symbolic-ref first (works when origin/HEAD is set)
	if out, err := run("symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		parts := strings.SplitN(out, "/", 4)
		if len(parts) == 4 {
			return parts[3], nil
		}
	}

	// Fallback: check which common branch exists locally
	for _, name := range []string{"main", "master"} {
		if err := exec.Command("git", "rev-parse", "--verify", name).Run(); err == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("could not detect default branch")
}

// LogRange returns one-line log entries between base and HEAD.
func (ExecGit) LogRange(base string) (string, error) {
	return run("log", "--oneline", base+"..HEAD")
}

// DiffRange returns the diff between the merge-base of base and HEAD.
func (ExecGit) DiffRange(base string) (string, error) {
	return run("diff", base+"...HEAD")
}

// DiffStatRange returns the diff stat between the merge-base of base and HEAD.
func (ExecGit) DiffStatRange(base string) (string, error) {
	return run("diff", "--stat", base+"...HEAD")
}

// HasUpstream checks whether the current branch has a remote tracking branch.
func (ExecGit) HasUpstream() bool {
	err := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}").Run()
	return err == nil
}

// Free functions delegate to Default for backward compatibility.

func StageAll() error                        { return Default.StageAll() }
func DiffStat() (string, error)              { return Default.DiffStat() }
func DiffCached() (string, error)            { return Default.DiffCached() }
func Commit(message string) (string, error)  { return Default.Commit(message) }
func Push() (string, error)                  { return Default.Push() }
func PushSetUpstream() (string, error)       { return Default.PushSetUpstream() }
func Reset() error                           { return Default.Reset() }
func Log() (string, error)                   { return Default.Log() }
func LogOneline() (string, error)            { return Default.LogOneline() }
func HasStagedChanges() bool                 { return Default.HasStagedChanges() }
func StatusShort() (string, error)           { return Default.StatusShort() }
func CurrentBranch() (string, error)         { return Default.CurrentBranch() }
func DefaultBranch() (string, error)         { return Default.DefaultBranch() }
func LogRange(base string) (string, error)   { return Default.LogRange(base) }
func DiffRange(base string) (string, error)  { return Default.DiffRange(base) }
func DiffStatRange(base string) (string, error) { return Default.DiffStatRange(base) }
func HasUpstream() bool                      { return Default.HasUpstream() }
