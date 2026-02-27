package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/git"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var messageFlag string

var rootCmd = &cobra.Command{
	Use:   "yeet [message...]",
	Short: "Git commit & push in one command",
	Long:  "Stage all changes, generate or use a commit message, and push — all in one step.",
	RunE:  runYeet,
}

func init() {
	rootCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Commit message (use when message collides with a subcommand name)")
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
		fmt.Println("\n  Nothing to commit.")
		return nil
	}
	fmt.Println()
	for _, line := range strings.Split(stat, "\n") {
		fmt.Println("  " + line)
	}
	fmt.Println()

	// 3. Get commit message
	var message string
	var usage *ai.Usage
	if messageFlag != "" {
		message = messageFlag
	} else if len(args) > 0 {
		message = strings.Join(args, " ")
	} else {
		message, usage, err = generateOrFallback()
		if err != nil {
			return err
		}
	}

	// 4. Confirm loop (show message, allow edit)
	for {
		fmt.Printf("  > %s\n\n", message)
		fmt.Println("  Enter commit  ·  e edit  ·  E $EDITOR  ·  Esc cancel")

		action, err := waitForAction()
		if err != nil {
			return err
		}

		switch action {
		case actionCancel:
			fmt.Println()
			if autoStaged {
				if err := git.Reset(); err != nil {
					return fmt.Errorf("failed to unstage changes: %w", err)
				}
			}
			fmt.Println("  Cancelled.")
			return nil
		case actionEdit:
			edited, err := editLine(message)
			if err != nil {
				return err
			}
			message = edited
			// clear prompt area and loop back
			fmt.Print("\033[2K\033[1A\033[2K\033[1A\033[2K\r")
			continue
		case actionEditExternal:
			fmt.Println()
			edited, err := editExternal(message)
			if err != nil {
				fmt.Printf("\n  Editor failed: %v\n", err)
			} else {
				message = edited
			}
			// clear prompt area and loop back
			fmt.Print("\033[2K\033[1A\033[2K\033[1A\033[2K\r")
			continue
		case actionConfirm:
			fmt.Println()
		}
		break
	}

	// 6. Commit
	out, err := git.Commit(message)
	if err != nil {
		return fmt.Errorf("commit failed: %s", out)
	}
	fmt.Println("  " + firstLine(out))

	// 7. Push
	pushOut, err := git.Push()
	if err != nil {
		// Try set-upstream if push fails
		pushOut, err = git.PushSetUpstream()
		if err != nil {
			return fmt.Errorf("push failed: %s", pushOut)
		}
	}

	branch, _ := git.CurrentBranch()
	fmt.Printf("  Pushed to origin/%s.\n", branch)

	if usage != nil && usage.InputTokens > 0 {
		costLine := fmt.Sprintf("  %s · %s", usage.FormatTokens(), usage.Model)
		if cost, ok := usage.Cost(); ok {
			costLine = fmt.Sprintf("  %s · %s · %s", cost, usage.FormatTokens(), usage.Model)
		}
		fmt.Println(costLine)
	}

	return nil
}

func generateOrFallback() (string, *ai.Usage, error) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	provider, providerErr := ai.NewProvider(cfg)

	// No provider available — offer quick setup or manual input
	if providerErr != nil {
		fmt.Printf("  No API key found for %s.\n\n", cfg.Provider)
		fmt.Println("  Set up AI now? (y/n)")

		yes, err := waitForYesNo()
		if err != nil {
			return "", nil, err
		}

		if yes {
			if err := quickSetup(cfg); err != nil {
				fmt.Printf("  Setup failed: %v\n\n", err)
			} else {
				// Retry with new key
				provider, providerErr = ai.NewProvider(cfg)
			}
		}

		// Still no provider — fall back to manual input
		if providerErr != nil {
			fmt.Println("  Enter commit message:")
			msg, err := editLine("")
			if err != nil {
				return "", nil, err
			}
			fmt.Println()
			if msg == "" {
				return "", nil, fmt.Errorf("empty commit message")
			}
			fmt.Printf("\n  tip: run `yeet auth set %s` to enable AI commit messages\n\n", cfg.Provider)
			return msg, nil, nil
		}
	}

	// AI generation
	fmt.Print("  Generating commit message...")

	diff, err := git.DiffCached()
	if err != nil {
		fmt.Println(" failed")
		return "", nil, fmt.Errorf("failed to get diff: %w", err)
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

	message, usage, err := provider.GenerateCommitMessage(ctx)
	if err != nil {
		// AI failed at runtime — fall back to manual
		fmt.Println(" failed")
		fmt.Printf("  %v\n\n", err)
		fmt.Println("  Enter commit message manually:")
		msg, editErr := editLine("")
		if editErr != nil {
			return "", nil, editErr
		}
		fmt.Println()
		if msg == "" {
			return "", nil, fmt.Errorf("empty commit message")
		}
		return msg, nil, nil
	}

	fmt.Print("\r\033[K") // clear the "Generating..." line
	return message, &usage, nil
}

func quickSetup(cfg config.Config) error {
	fmt.Printf("\n  Enter API key for %s: ", cfg.Provider)
	key, err := term.ReadPassword(int(os.Stdin.Fd()))
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

	fmt.Printf("  ✓ Key saved for %s.\n\n", cfg.Provider)
	return nil
}

func waitForYesNo() (bool, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return false, err
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buf)
		if err != nil {
			return false, err
		}
		switch buf[0] {
		case 'y', 'Y', 13, 10: // y or Enter
			return true, nil
		case 'n', 'N', 27, 3: // n, Esc, Ctrl+C
			return false, nil
		}
	}
}

type action int

const (
	actionConfirm action = iota
	actionCancel
	actionEdit
	actionEditExternal
)

func waitForAction() (action, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return actionCancel, fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.Restore(fd, oldState)

	buf := make([]byte, 3)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return actionCancel, err
		}
		for i := 0; i < n; i++ {
			switch buf[i] {
			case 13, 10: // Enter
				return actionConfirm, nil
			case 27: // Escape
				return actionCancel, nil
			case 3: // Ctrl+C
				return actionCancel, nil
			case 'e':
				return actionEdit, nil
			case 'E':
				return actionEditExternal, nil
			}
		}
	}
}

func editLine(initial string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return initial, fmt.Errorf("failed to set raw terminal: %w", err)
	}
	defer term.Restore(fd, oldState)

	line := []rune(initial)
	cursor := len(line)

	redraw := func() {
		fmt.Print("\r\033[2K")
		fmt.Printf("  > %s", string(line))
		// move cursor to correct position
		back := len(line) - cursor
		if back > 0 {
			fmt.Printf("\033[%dD", back)
		}
	}

	redraw()

	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return string(line), err
		}

		for i := 0; i < n; {
			b := buf[i]
			switch {
			case b == 13 || b == 10: // Enter — confirm edit
				fmt.Print("\r\033[2K")
				return string(line), nil
			case b == 27: // Escape sequence
				if i+2 < n && buf[i+1] == '[' {
					switch buf[i+2] {
					case 'C': // Right
						if cursor < len(line) {
							cursor++
						}
					case 'D': // Left
						if cursor > 0 {
							cursor--
						}
					case 'H': // Home
						cursor = 0
					case 'F': // End
						cursor = len(line)
					case '3': // Delete (ESC [ 3 ~)
						if cursor < len(line) {
							line = append(line[:cursor], line[cursor+1:]...)
						}
						if i+3 < n && buf[i+3] == '~' {
							i++ // consume the ~
						}
					}
					i += 3
					redraw()
					continue
				}
				// bare Escape — cancel edit, return original
				fmt.Print("\r\033[2K")
				return initial, nil
			case b == 127 || b == 8: // Backspace
				if cursor > 0 {
					line = append(line[:cursor-1], line[cursor:]...)
					cursor--
				}
			case b == 1: // Ctrl+A — home
				cursor = 0
			case b == 5: // Ctrl+E — end
				cursor = len(line)
			case b == 21: // Ctrl+U — clear line
				line = line[:0]
				cursor = 0
			case b == 3: // Ctrl+C — cancel
				fmt.Print("\r\033[2K")
				return initial, nil
			case b >= 32 && b < 127: // printable ASCII
				line = append(line[:cursor], append([]rune{rune(b)}, line[cursor:]...)...)
				cursor++
			}
			i++
			redraw()
		}
	}
}

func getEditor() string {
	if e := os.Getenv("VISUAL"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

func editExternal(initial string) (string, error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "yeet-commit-msg.txt")
	if err := os.WriteFile(tmpFile, []byte(initial), 0600); err != nil {
		return initial, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)

	editor := getEditor()
	cmd := exec.Command(editor, tmpFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return initial, fmt.Errorf("editor exited with error: %w", err)
	}

	content, err := os.ReadFile(tmpFile)
	if err != nil {
		return initial, fmt.Errorf("failed to read edited file: %w", err)
	}

	edited := strings.TrimSpace(string(content))
	if edited == "" {
		return initial, nil
	}
	return edited, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
