package forge

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// lookPath, run and remoteURL are seams so forge behavior is unit-testable
// without real CLIs or a real repository. run shells out to a command and
// returns combined output.
var (
	lookPath = exec.LookPath
	run      = func(name string, args ...string) (string, error) {
		out, err := exec.Command(name, args...).CombinedOutput()
		return string(out), err
	}
	remoteURL = func() (string, error) {
		return run("git", "remote", "get-url", "origin")
	}
)

// Forge abstracts GitHub and GitLab PR/MR operations.
type Forge interface {
	Name() string
	CLIName() string
	ExistingPR(branch string) (url string, exists bool)
	CreatePR(title, body, base string) (url string, err error)
}

// Detect returns the appropriate Forge for the current repository.
//
// Remotes that mention "gitlab" always map to GitLab (glab required).
// Everything else prefers gh; if only glab is installed, GitLab is assumed —
// self-hosted GitLab hosts often do not carry "gitlab" in their hostname.
func Detect() (Forge, error) {
	remote, err := remoteURL()
	if err != nil {
		return nil, fmt.Errorf("no git remote 'origin' found")
	}

	_, glabErr := lookPath("glab")
	_, ghErr := lookPath("gh")
	glabFound := glabErr == nil
	ghFound := ghErr == nil

	if strings.Contains(remote, "gitlab") {
		if !glabFound {
			return nil, fmt.Errorf("GitLab remote detected but 'glab' CLI is not installed")
		}
		return GitLab{}, nil
	}

	switch {
	case ghFound:
		return GitHub{}, nil
	case glabFound:
		return GitLab{}, nil
	default:
		return nil, fmt.Errorf("neither 'gh' nor 'glab' CLI is installed")
	}
}

// --- GitHub ---

// GitHub implements Forge using the gh CLI.
type GitHub struct{}

func (GitHub) Name() string    { return "GitHub" }
func (GitHub) CLIName() string { return "gh" }

func (GitHub) ExistingPR(branch string) (string, bool) {
	out, err := run("gh", "pr", "view", branch, "--json", "url", "--jq", ".url")
	if err != nil {
		return "", false
	}
	url := strings.TrimSpace(out)
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
	out, err := run("gh", args...)
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// --- GitLab ---

// GitLab implements Forge using the glab CLI.
type GitLab struct{}

func (GitLab) Name() string    { return "GitLab" }
func (GitLab) CLIName() string { return "glab" }

func (GitLab) ExistingPR(branch string) (string, bool) {
	out, err := run("glab", "mr", "view", branch, "--output", "json")
	if err == nil {
		var mr struct {
			WebURL string `json:"web_url"`
		}
		if json.Unmarshal([]byte(out), &mr) == nil && mr.WebURL != "" {
			return mr.WebURL, true
		}
		// Structured response without web_url means no MR for this branch.
		trimmed := strings.TrimSpace(out)
		if strings.HasPrefix(trimmed, "{") || trimmed == "null" {
			return "", false
		}
	}
	// Fall back to plain view for old glab versions whose --output flag
	// fails or prints non-JSON.
	viewOut, err := run("glab", "mr", "view", branch)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(viewOut, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"url:", "URL:"} {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), true
			}
		}
	}
	return "", true
}

func (GitLab) CreatePR(title, body, base string) (string, error) {
	args := []string{"mr", "create", "--fill", "--title", title, "--description", body}
	if base != "" {
		args = append(args, "--target-branch", base)
	}
	out, err := run("glab", args...)
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(out))
	}
	// Extract URL from glab output
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http") {
			return line, nil
		}
	}
	return strings.TrimSpace(out), nil
}
