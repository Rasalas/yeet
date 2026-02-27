package forge

import (
	"fmt"
	"os/exec"
	"strings"
)

// Forge abstracts GitHub and GitLab PR/MR operations.
type Forge interface {
	Name() string
	CLIName() string
	ExistingPR(branch string) (url string, exists bool)
	CreatePR(title, body, base string) (url string, err error)
}

// Detect returns the appropriate Forge for the current repository.
// It inspects the origin remote URL and checks for the corresponding CLI tool.
func Detect() (Forge, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("no git remote 'origin' found")
	}

	remote := strings.TrimSpace(string(out))

	if strings.Contains(remote, "gitlab") {
		if _, err := exec.LookPath("glab"); err != nil {
			return nil, fmt.Errorf("GitLab remote detected but 'glab' CLI is not installed")
		}
		return GitLab{}, nil
	}

	if _, err := exec.LookPath("gh"); err != nil {
		return nil, fmt.Errorf("GitHub remote detected but 'gh' CLI is not installed")
	}
	return GitHub{}, nil
}

// --- GitHub ---

// GitHub implements Forge using the gh CLI.
type GitHub struct{}

func (GitHub) Name() string    { return "GitHub" }
func (GitHub) CLIName() string { return "gh" }

func (GitHub) ExistingPR(branch string) (string, bool) {
	out, err := exec.Command("gh", "pr", "view", branch, "--json", "url", "--jq", ".url").CombinedOutput()
	if err != nil {
		return "", false
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", false
	}
	return url, true
}

func (GitHub) CreatePR(title, body, base string) (string, error) {
	args := []string{"pr", "create", "--title", title, "--body", body}
	if base != "" {
		args = append(args, "--base", base)
	}
	out, err := exec.Command("gh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// --- GitLab ---

// GitLab implements Forge using the glab CLI.
type GitLab struct{}

func (GitLab) Name() string    { return "GitLab" }
func (GitLab) CLIName() string { return "glab" }

func (GitLab) ExistingPR(branch string) (string, bool) {
	out, err := exec.Command("glab", "mr", "view", branch, "--output", "json").CombinedOutput()
	if err != nil {
		return "", false
	}
	// glab mr view exits 0 and returns JSON when MR exists
	text := strings.TrimSpace(string(out))
	if text == "" || strings.Contains(text, "no open merge request") {
		return "", false
	}
	// Extract web_url from output — glab prints it on a "URL:" line in default mode
	// Use --output json not available on all versions, fall back to plain view
	viewOut, err := exec.Command("glab", "mr", "view", branch).CombinedOutput()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(viewOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "url:") || strings.HasPrefix(line, "URL:") {
			return strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), true
		}
	}
	return "", true
}

func (GitLab) CreatePR(title, body, base string) (string, error) {
	args := []string{"mr", "create", "--fill", "--title", title, "--description", body}
	if base != "" {
		args = append(args, "--target-branch", base)
	}
	out, err := exec.Command("glab", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	// Extract URL from glab output
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") {
			return line, nil
		}
	}
	return strings.TrimSpace(string(out)), nil
}
