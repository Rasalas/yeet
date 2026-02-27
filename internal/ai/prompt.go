package ai

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultPrompt is the built-in system prompt for commit message generation.
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

// PRPrompt is the system prompt for generating PR/MR titles and descriptions.
const PRPrompt = `You are a pull request description generator. Given branch commits and a diff, generate a PR title and markdown body.

Rules:
- First line: a concise PR title (under 72 characters, no prefix like "PR:" or "feat:")
- Second line: empty
- Remaining lines: markdown body with a "## Summary" section containing 2-5 bullet points
- Bullet points should describe WHAT changed and WHY from an end-user or reviewer perspective
- Match the language and style of the commit messages when provided
- Do NOT include a test plan or checklist — only the summary
- Return ONLY the title and body, nothing else — no quotes, no explanation`

const maxDiffLines = 8000

// PromptPath returns the path to the user's prompt file.
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
		WritePrompt(DefaultPrompt)
		return DefaultPrompt
	}

	prompt := strings.TrimSpace(string(data))
	if prompt == "" {
		return DefaultPrompt
	}
	return prompt
}

// WritePrompt writes the prompt content to the prompt file.
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
