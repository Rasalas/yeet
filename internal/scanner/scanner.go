package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RepoLog holds git log data for a single repository.
type RepoLog struct {
	Name string // relative display name, e.g. "web/backend"
	Path string // absolute path to repo root
	Log  string // formatted commit log
	Stat string // diff stat summary
}

const maxDepth = 5

// FindRepos walks each path and returns a list of git repo root directories.
// Paths can be individual repos or parent directories containing repos.
func FindRepos(paths []string) ([]string, error) {
	var repos []string
	seen := make(map[string]bool)

	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("invalid path %s: %w", p, err)
		}

		info, err := os.Stat(abs)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", abs, err)
		}
		if !info.IsDir() {
			continue
		}

		// Check if this path itself is a repo
		if isGitRepo(abs) {
			if !seen[abs] {
				seen[abs] = true
				repos = append(repos, abs)
			}
			continue
		}

		// Walk subdirectories up to maxDepth
		walkRepos(abs, abs, 0, seen, &repos)
	}

	return repos, nil
}

func walkRepos(root, dir string, depth int, seen map[string]bool, repos *[]string) {
	if depth > maxDepth {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
			continue
		}

		sub := filepath.Join(dir, name)
		if isGitRepo(sub) {
			if !seen[sub] {
				seen[sub] = true
				*repos = append(*repos, sub)
			}
			continue
		}

		walkRepos(root, sub, depth+1, seen, repos)
	}
}

func isGitRepo(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil && info.IsDir()
}

// CollectLogs runs git log/stat commands on each repo and returns results.
// Repos with no matching commits are skipped.
func CollectLogs(repos []string, since, until, author string) ([]RepoLog, error) {
	var results []RepoLog

	for _, repo := range repos {
		log, err := gitLog(repo, since, until, author)
		if err != nil || strings.TrimSpace(log) == "" {
			continue
		}

		stat, _ := gitStat(repo, since, until, author)

		results = append(results, RepoLog{
			Name: repoDisplayName(repo),
			Path: repo,
			Log:  strings.TrimSpace(log),
			Stat: strings.TrimSpace(stat),
		})
	}

	return results, nil
}

func gitLog(repo, since, until, author string) (string, error) {
	args := []string{"-C", repo, "log",
		"--since=" + since, "--until=" + until,
		"--format=%ad %s", "--date=short",
	}
	if author != "" {
		args = append(args, "--author="+author)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	return string(out), err
}

func gitStat(repo, since, until, author string) (string, error) {
	args := []string{"-C", repo, "log",
		"--since=" + since, "--until=" + until,
		"--stat", "--format=commit %h %s",
	}
	if author != "" {
		args = append(args, "--author="+author)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	return string(out), err
}

// repoDisplayName returns a short display name derived from the last two path components.
func repoDisplayName(repo string) string {
	parent := filepath.Base(filepath.Dir(repo))
	base := filepath.Base(repo)
	if parent == "." || parent == "/" {
		return base
	}
	return parent + "/" + base
}

// DefaultAuthor returns the git user.name from global config.
func DefaultAuthor() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
