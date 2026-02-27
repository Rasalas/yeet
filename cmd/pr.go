package cmd

import (
	"fmt"
	"strings"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/forge"
	"github.com/rasalas/yeet/internal/git"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(prCmd)
}

var prCmd = &cobra.Command{
	Use:          "pr",
	Short:        "Create a pull request with an AI-generated description",
	Long:         "Detect the forge (GitHub/GitLab), collect branch commits, generate a PR title and body via AI, and create the PR/MR.",
	SilenceUsage: true,
	RunE:         runPR,
}

func runPR(cmd *cobra.Command, args []string) error {
	// 1. Detect forge
	f, err := forge.Detect()
	if err != nil {
		return err
	}

	// 2. Check we're not on the default branch
	branch, err := git.CurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	base, err := git.DefaultBranch()
	if err != nil {
		return fmt.Errorf("failed to detect default branch: %w", err)
	}

	if branch == base {
		return fmt.Errorf("already on %s — switch to a feature branch first", base)
	}

	// 3. Push if no upstream
	if !git.HasUpstream() {
		fmt.Printf("\n  %sPushing %s to origin...%s\n", term.Dim, branch, term.Reset)
		out, err := git.PushSetUpstream()
		if err != nil {
			return fmt.Errorf("push failed: %s", out)
		}
		fmt.Printf("  %s✓%s pushed to origin/%s\n", term.Green, term.Reset, branch)
	}

	// 4. Check for existing PR/MR
	if url, exists := f.ExistingPR(branch); exists {
		if url != "" {
			fmt.Printf("\n  A %s PR already exists: %s\n\n", f.Name(), url)
		} else {
			fmt.Printf("\n  A %s PR already exists for branch %s.\n\n", f.Name(), branch)
		}
		return nil
	}

	// 5. Check for uncommitted changes
	status, _ := git.StatusShort()
	if status != "" {
		fmt.Printf("\n  %sUncommitted changes detected — run %syeet%s first, then %syeet pr%s.%s\n\n",
			term.Dim, term.Reset+term.Bold, term.Reset+term.Dim, term.Reset+term.Bold, term.Reset+term.Dim, term.Reset)
		return nil
	}

	// 6. Collect commits + diff
	commits, err := git.LogRange(base)
	if err != nil || commits == "" {
		fmt.Printf("\n  %sNo commits between %s and %s — nothing to open a PR for.%s\n\n", term.Dim, base, branch, term.Reset)
		return nil
	}

	diff, err := git.DiffRange(base)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	stat, err := git.DiffStatRange(base)
	if err != nil {
		return fmt.Errorf("failed to get diff stat: %w", err)
	}

	// Show diff stat
	fmt.Println()
	for _, line := range strings.Split(stat, "\n") {
		fmt.Println("  " + term.ColorizeDiffStat(line))
	}
	fmt.Println()

	// 6. AI generation
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	provider, err := ai.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("no AI provider configured: %w", err)
	}

	ctx := ai.CommitContext{
		Diff:          diff,
		Branch:        branch,
		RecentCommits: commits,
		SystemPrompt:  ai.PRPrompt,
		MaxTokens:     1024,
	}

	var title, body string
	var usage *ai.Usage

	var s term.Spinner
	s.Start("Generating PR description...")

	msg, u, genErr := provider.GenerateCommitMessage(ctx)
	s.Stop()

	if genErr != nil {
		return fmt.Errorf("AI generation failed: %w", genErr)
	}
	usage = &u

	title, body = parsePR(msg)

	// 7. Preview
	showPRPreview := true
	linesToClear := 3
	for {
		if showPRPreview {
			linesToClear = displayPRPreview(title, body)
		} else {
			showPRPreview = true
		}

		fmt.Printf("  %s%s  ·  %s  ·  %s  ·  %s%s\n",
			term.Dim,
			term.Keyhint("enter", "create"),
			term.Keyhint("e", "edit title"),
			term.Keyhint("E", "editor"),
			term.Keyhint("q", "cancel"),
			term.Reset)

		action, err := term.WaitForAction()
		if err != nil {
			return err
		}

		switch action {
		case term.ActionCancel:
			fmt.Printf("\n  %sCancelled.%s\n", term.Dim, term.Reset)
			return nil

		case term.ActionEdit:
			term.ClearLines(linesToClear)
			edited, err := term.EditLine(title)
			if err != nil {
				return err
			}
			title = edited
			continue

		case term.ActionEditExternal:
			term.ClearLines(linesToClear)
			full := title + "\n\n" + body
			edited, err := term.EditExternal(full)
			if err != nil {
				fmt.Printf("\n  Editor failed: %v\n", err)
			} else {
				title, body = parsePR(edited)
			}
			continue

		case term.ActionConfirm:
			fmt.Println()
		}
		break
	}

	// 8. Create PR
	url, err := f.CreatePR(title, body, base)
	if err != nil {
		return fmt.Errorf("failed to create %s PR: %w", f.Name(), err)
	}

	fmt.Printf("  %s✓%s %s PR created: %s\n", term.Green, term.Reset, f.Name(), url)

	// 9. Usage/cost
	if usage != nil && usage.InputTokens > 0 {
		costLine := fmt.Sprintf("%s · %s", usage.FormatTokens(), usage.Model)
		if cost, ok := usage.Cost(); ok {
			costLine = fmt.Sprintf("%s · %s · %s", cost, usage.FormatTokens(), usage.Model)
		}
		fmt.Printf("\n  %s%s%s\n\n", term.Dim, costLine, term.Reset)
	}

	return nil
}

// parsePR splits the AI output into title (first line) and body (rest).
func parsePR(raw string) (title, body string) {
	raw = strings.TrimSpace(raw)
	if i := strings.Index(raw, "\n"); i >= 0 {
		title = strings.TrimSpace(raw[:i])
		body = strings.TrimSpace(raw[i+1:])
	} else {
		title = raw
	}
	return
}

// displayPRPreview shows the PR title and body inside a single card.
// Returns the number of terminal lines used (for ClearLines).
func displayPRPreview(title, body string) int {
	lines := 0

	// Collect all content lines: title + blank + body
	var content []string
	content = append(content, "# "+title)
	if body != "" {
		content = append(content, "")
		content = append(content, strings.Split(body, "\n")...)
	}

	// Find the widest line for padding
	maxWidth := 0
	for _, line := range content {
		if w := len([]rune(line)); w > maxWidth {
			maxWidth = w
		}
	}

	if term.MsgBg != "" {
		pad := strings.Repeat(" ", maxWidth+3)
		// Top border
		fmt.Printf("  %s%s%s\n", term.MsgBar, pad, term.Reset)
		lines++
		// Content lines
		for i, line := range content {
			rpad := strings.Repeat(" ", maxWidth-len([]rune(line)))
			if i == 0 {
				// Title: bold
				fmt.Printf("  %s%s%s%s\n", term.MsgOpen, line, rpad, term.MsgClose)
			} else {
				// Body: dim on card bg
				fmt.Printf("  %s%s%s%s %s%s\n", term.MsgBar, term.Dim, line, rpad, term.Reset, "")
			}
			lines++
		}
		// Bottom border
		fmt.Printf("  %s%s%s\n", term.MsgBar, pad, term.Reset)
		lines++
	} else {
		// NO_COLOR fallback
		for i, line := range content {
			if i == 0 {
				fmt.Printf("  %s%s%s\n", term.MsgOpen, line, term.MsgClose)
			} else {
				fmt.Printf("  %s\n", line)
			}
			lines++
		}
	}

	fmt.Println()
	lines++ // blank line before keyhints
	lines++ // keyhint line itself
	return lines
}
