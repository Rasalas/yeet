package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func StageAll() error {
	_, err := run("add", "--all")
	return err
}

func DiffStat() (string, error) {
	return run("diff", "--cached", "--stat")
}

func DiffCached() (string, error) {
	return run("diff", "--cached")
}

func Commit(message string) (string, error) {
	return run("commit", "-m", message)
}

func Push() (string, error) {
	return run("push")
}

func Reset() error {
	_, err := run("reset")
	return err
}

func Log() (string, error) {
	return run("log", "--oneline", "--graph", "--decorate", "-20")
}

func LogOneline() (string, error) {
	return run("log", "--oneline", "-10")
}

func HasStagedChanges() bool {
	out, err := run("diff", "--cached", "--quiet")
	// exit code 1 = there are differences (staged changes exist)
	return err != nil && out == ""
}

func StatusShort() (string, error) {
	return run("status", "--short")
}

func CurrentBranch() (string, error) {
	return run("rev-parse", "--abbrev-ref", "HEAD")
}

func HasRemote() bool {
	out, err := run("remote")
	return err == nil && strings.TrimSpace(out) != ""
}

func PushSetUpstream() (string, error) {
	branch, err := CurrentBranch()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return run("push", "--set-upstream", "origin", branch)
}
