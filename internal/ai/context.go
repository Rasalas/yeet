package ai

import (
	"fmt"
	"strings"
)

// CommitContext holds all the information sent to the AI provider.
type CommitContext struct {
	Diff          string
	Branch        string
	RecentCommits string
	Status        string
}

// BuildUserMessage assembles the user message sent to the AI from git context.
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
