package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/git"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
	goterm "golang.org/x/term"
)

var messageFlag string

func init() {
	rootCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Commit message (use when message collides with a subcommand name)")
}

var rootCmd = &cobra.Command{
	Use:   "yeet [message...]",
	Short: "Git commit & push in one command",
	Long:  "Stage all changes, generate or use a commit message, and push — all in one step.",
	RunE:  runYeet,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// RunAsCommit is called by subcommands when they receive unexpected args,
// indicating the user meant it as a commit message, not a subcommand.
func RunAsCommit(subcommandName string, args []string) error {
	allArgs := append([]string{subcommandName}, args...)
	return runYeet(rootCmd, allArgs)
}

func runYeet(cmd *cobra.Command, args []string) error {
	// 1. Stage: respect existing staged changes, otherwise stage all
	autoStaged := false
	if !git.HasStagedChanges() {
		if err := git.StageAll(); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
		autoStaged = true
	}

	// 2. Show diff stat
	stat, err := git.DiffStat()
	if err != nil {
		return fmt.Errorf("failed to get diff stat: %w", err)
	}
	if stat == "" {
		fmt.Printf("\n  %sNothing to commit.%s\n", term.Dim, term.Reset)
		return nil
	}
	fmt.Println()
	for _, line := range strings.Split(stat, "\n") {
		fmt.Println("  " + term.ColorizeDiffStat(line))
	}
	fmt.Println()

	// 3. Get commit message
	var message string
	var usage *ai.Usage
	streamed := false
	if messageFlag != "" {
		message = messageFlag
	} else if len(args) > 0 {
		message = strings.Join(args, " ")
	} else {
		message, usage, streamed, err = generateOrFallback()
		if err != nil {
			return err
		}
	}

	// 4. Confirm loop (show message, allow edit)
	showMessage := !streamed
	linesToClear := 3
	if streamed && term.MsgBg != "" {
		term.ClearLines(1)
		showMessage = true
	}
	for {
		if showMessage {
			if term.MsgBg != "" {
				pad := strings.Repeat(" ", len([]rune(message))+3)
				fmt.Printf("  %s%s%s\n", term.MsgBar, pad, term.Reset)
				fmt.Printf("  %s%s%s\n", term.MsgOpen, message, term.MsgClose)
				fmt.Printf("  %s%s%s\n\n", term.MsgBar, pad, term.Reset)
				linesToClear = 5
			} else {
				fmt.Printf("  %s%s%s\n\n", term.MsgOpen, message, term.MsgClose)
				linesToClear = 3
			}
		} else {
			fmt.Println()
			showMessage = true
			linesToClear = 3
		}
		fmt.Printf("  %s%s  \u00b7  %s  \u00b7  %s  \u00b7  %s%s\n",
			term.Dim,
			term.Keyhint("enter", "commit"),
			term.Keyhint("e", "edit"),
			term.Keyhint("E", "editor"),
			term.Keyhint("q", "cancel"),
			term.Reset)

		action, err := term.WaitForAction()
		if err != nil {
			return err
		}

		switch action {
		case term.ActionCancel:
			fmt.Println()
			if autoStaged {
				if err := git.Reset(); err != nil {
					return fmt.Errorf("failed to unstage changes: %w", err)
				}
			}
			fmt.Printf("  %sCancelled.%s\n", term.Dim, term.Reset)
			return nil
		case term.ActionEdit:
			term.ClearLines(linesToClear)
			edited, err := term.EditLine(message)
			if err != nil {
				return err
			}
			message = edited
			continue
		case term.ActionEditExternal:
			term.ClearLines(linesToClear)
			edited, err := term.EditExternal(message)
			if err != nil {
				fmt.Printf("\n  Editor failed: %v\n", err)
			} else {
				message = edited
			}
			continue
		case term.ActionConfirm:
			fmt.Println()
		}
		break
	}

	// 6. Commit
	out, err := git.Commit(message)
	if err != nil {
		return fmt.Errorf("commit failed: %s", out)
	}
	fmt.Printf("  %s\u2713%s %s\n", term.Green, term.Reset, firstLine(out))

	// 7. Push
	pushOut, err := git.Push()
	if err != nil {
		pushOut, err = git.PushSetUpstream()
		if err != nil {
			return fmt.Errorf("push failed: %s", pushOut)
		}
	}

	branch, _ := git.CurrentBranch()
	fmt.Printf("  %s\u2713%s %spushed to%s origin/%s\n", term.Green, term.Reset, term.Dim, term.Reset, branch)

	if usage != nil && usage.InputTokens > 0 {
		costLine := fmt.Sprintf("%s \u00b7 %s", usage.FormatTokens(), usage.Model)
		if cost, ok := usage.Cost(); ok {
			costLine = fmt.Sprintf("%s \u00b7 %s \u00b7 %s", cost, usage.FormatTokens(), usage.Model)
		}
		fmt.Printf("\n  %s%s%s\n\n", term.Dim, costLine, term.Reset)
	}

	return nil
}

func generateOrFallback() (string, *ai.Usage, bool, error) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	provider, providerErr := ai.NewProvider(cfg)

	if providerErr != nil {
		fmt.Printf("  No API key found for %s.\n\n", cfg.Provider)
		fmt.Println("  Set up AI now? (y/n)")

		yes, err := term.WaitForYesNo()
		if err != nil {
			return "", nil, false, err
		}

		if yes {
			if err := quickSetup(cfg); err != nil {
				fmt.Printf("  Setup failed: %v\n\n", err)
			} else {
				provider, providerErr = ai.NewProvider(cfg)
			}
		}

		if providerErr != nil {
			fmt.Println("  Enter commit message:")
			msg, err := term.EditLine("")
			if err != nil {
				return "", nil, false, err
			}
			fmt.Println()
			if msg == "" {
				return "", nil, false, fmt.Errorf("empty commit message")
			}
			fmt.Printf("\n  tip: run `yeet auth set %s` to enable AI commit messages\n\n", cfg.Provider)
			return msg, nil, false, nil
		}
	}

	// AI generation — collect git context
	diff, err := git.DiffCached()
	if err != nil {
		return "", nil, false, fmt.Errorf("failed to get diff: %w", err)
	}

	branch, _ := git.CurrentBranch()
	recentLog, _ := git.LogOneline()
	status, _ := git.StatusShort()

	ctx := ai.CommitContext{
		Diff:          diff,
		Branch:        branch,
		RecentCommits: recentLog,
		Status:        status,
	}

	// Try streaming if supported
	if sp, ok := provider.(ai.StreamingProvider); ok {
		message, usage, err := generateStreaming(sp, ctx)
		if err != nil {
			term.ClearLine()
			fmt.Printf("  %s%v%s\n\n", term.Red, err, term.Reset)
			fmt.Println("  Enter commit message manually:")
			msg, editErr := term.EditLine(message)
			if editErr != nil {
				return "", nil, false, editErr
			}
			fmt.Println()
			if msg == "" {
				return "", nil, false, fmt.Errorf("empty commit message")
			}
			return msg, nil, false, nil
		}
		return message, &usage, true, nil
	}

	// Non-streaming fallback
	fmt.Printf("  %sGenerating commit message...%s", term.Dim, term.Reset)
	message, usage, err := provider.GenerateCommitMessage(ctx)
	if err != nil {
		fmt.Println(" failed")
		fmt.Printf("  %s%v%s\n\n", term.Red, err, term.Reset)
		fmt.Println("  Enter commit message manually:")
		msg, editErr := term.EditLine("")
		if editErr != nil {
			return "", nil, false, editErr
		}
		fmt.Println()
		if msg == "" {
			return "", nil, false, fmt.Errorf("empty commit message")
		}
		return msg, nil, false, nil
	}

	term.ClearLine()
	return message, &usage, false, nil
}

func generateStreaming(sp ai.StreamingProvider, ctx ai.CommitContext) (string, ai.Usage, error) {
	var s term.Spinner
	s.Start("Generating...")

	message, usage, err := sp.GenerateCommitMessageStream(ctx, func(token string) {
		s.ReplaceWithContent(token)
	})

	s.Stop()
	return message, usage, err
}

func quickSetup(cfg config.Config) error {
	fmt.Printf("\n  Enter API key for %s: ", cfg.Provider)
	key, err := goterm.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}

	apiKey := strings.TrimSpace(string(key))
	if apiKey == "" {
		return fmt.Errorf("empty key")
	}

	if err := keyring.Set(cfg.Provider, apiKey); err != nil {
		return fmt.Errorf("failed to save key: %w", err)
	}

	fmt.Printf("  %s\u2713%s Key saved for %s.\n\n", term.Green, term.Reset, cfg.Provider)
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
