package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultPrompt = `You are a commit message generator. Given git context, generate a single conventional commit message.

Rules:
- Use conventional commit format: type(scope): description
- Types: feat, fix, refactor, docs, style, test, chore, build, ci, perf
- Scope is optional, use it when changes are focused on one area
- Description should be lowercase, imperative mood, no period at the end
- Keep the message under 72 characters
- Prefer to explain WHY something was done from an end user perspective instead of WHAT was done
- Be specific about what user-facing changes were made — avoid generic messages
- Match the style and language of the recent commits when provided
- Use the branch name as a hint for type and scope when relevant
- Return ONLY the commit message, nothing else — no quotes, no explanation`

const maxDiffLines = 8000

func PromptPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "yeet", "prompt.txt"), nil
}

// LoadPrompt reads the prompt from ~/.config/yeet/prompt.txt.
// If the file doesn't exist, it creates it with the default prompt.
func LoadPrompt() string {
	path, err := PromptPath()
	if err != nil {
		return DefaultPrompt
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist — create it with the default
		WritePrompt(DefaultPrompt)
		return DefaultPrompt
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return DefaultPrompt
	}
	return prompt
}

func WritePrompt(content string) error {
	path, err := PromptPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content+"\n"), 0644)
}

// CommitContext holds all the information sent to the AI provider.
type CommitContext struct {
	Diff          string
	Branch        string
	RecentCommits string
	Status        string
}

func (c CommitContext) BuildUserMessage() string {
	var b strings.Builder

	if c.Branch != "" {
		fmt.Fprintf(&b, "Branch: %s\n\n", c.Branch)
	}

	if c.Status != "" {
		fmt.Fprintf(&b, "Files changed:\n%s\n\n", c.Status)
	}

	if c.RecentCommits != "" {
		fmt.Fprintf(&b, "Recent commits:\n%s\n\n", c.RecentCommits)
	}

	b.WriteString("Diff:\n")
	b.WriteString(truncateDiff(c.Diff))

	return b.String()
}

func truncateDiff(diff string) string {
	lines := 0
	for i, c := range diff {
		if c == '\n' {
			lines++
			if lines >= maxDiffLines {
				return diff[:i] + "\n... (diff truncated)"
			}
		}
	}
	return diff
}
