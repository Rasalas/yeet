package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/git"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ANSI color codes — disabled when NO_COLOR is set.
var (
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	purple = "\033[35m"
	reset  = "\033[0m"
)

// keyhint formats a keybinding: key in bold, description in dim.
func keyhint(key, desc string) string {
	return reset + bold + key + reset + dim + " " + desc
}

func init() {
	if os.Getenv("NO_COLOR") != "" {
		bold = ""
		dim = ""
		red = ""
		green = ""
		purple = ""
		reset = ""
	}

	rootCmd.Flags().StringVarP(&messageFlag, "message", "m", "", "Commit message (use when message collides with a subcommand name)")
}

// diffStatRe matches lines like "  file.go | 29 ++---"
var diffStatRe = regexp.MustCompile(`^(.*\|[^+-]*?)(\+*)(-*)$`)

var messageFlag string

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
		fmt.Printf("\n  %sNothing to commit.%s\n", dim, reset)
		return nil
	}
	fmt.Println()
	for _, line := range strings.Split(stat, "\n") {
		fmt.Println("  " + colorizeDiffStat(line))
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
	showMessage := !streamed // skip first display if already streamed
	for {
		if showMessage {
			fmt.Printf("  %s%s› %s%s\n\n", bold, purple, message, reset)
		} else {
			fmt.Println()
			showMessage = true // always show on subsequent iterations (after edit)
		}
		fmt.Printf("  %s%s  ·  %s  ·  %s  ·  %s%s\n",
			dim,
			keyhint("enter", "commit"),
			keyhint("e", "edit"),
			keyhint("E", "editor"),
			keyhint("esc", "cancel"),
			reset)

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
			fmt.Printf("  %sCancelled.%s\n", dim, reset)
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
	fmt.Printf("  %s✓%s %s\n", green, reset, firstLine(out))

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
	fmt.Printf("  %s✓%s %spushed to%s origin/%s\n", green, reset, dim, reset, branch)

	if usage != nil && usage.InputTokens > 0 {
		costLine := fmt.Sprintf("%s · %s", usage.FormatTokens(), usage.Model)
		if cost, ok := usage.Cost(); ok {
			costLine = fmt.Sprintf("%s · %s · %s", cost, usage.FormatTokens(), usage.Model)
		}
		fmt.Printf("  %s%s%s\n", dim, costLine, reset)
	}

	return nil
}

func generateOrFallback() (string, *ai.Usage, bool, error) {
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
			return "", nil, false, err
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

	// Try streaming if supported, otherwise fall back to non-streaming
	if sp, ok := provider.(ai.StreamingProvider); ok {
		message, usage, err := generateStreaming(sp, ctx)
		if err != nil {
			fmt.Print("\r\033[K")
			fmt.Printf("  %s%v%s\n\n", red, err, reset)
			fmt.Println("  Enter commit message manually:")
			msg, editErr := editLine("")
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
	fmt.Printf("  %sGenerating commit message...%s", dim, reset)
	message, usage, err := provider.GenerateCommitMessage(ctx)
	if err != nil {
		fmt.Println(" failed")
		fmt.Printf("  %s%v%s\n\n", red, err, reset)
		fmt.Println("  Enter commit message manually:")
		msg, editErr := editLine("")
		if editErr != nil {
			return "", nil, false, editErr
		}
		fmt.Println()
		if msg == "" {
			return "", nil, false, fmt.Errorf("empty commit message")
		}
		return msg, nil, false, nil
	}

	fmt.Print("\r\033[K") // clear the "Generating..." line
	return message, &usage, false, nil
}

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func generateStreaming(sp ai.StreamingProvider, ctx ai.CommitContext) (string, ai.Usage, error) {
	// Start spinner
	var mu sync.Mutex
	firstToken := false
	spinnerDone := make(chan struct{})

	go func() {
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		// Show initial spinner frame immediately
		mu.Lock()
		if !firstToken {
			fmt.Printf("\r  %s%c Generating...%s", dim, spinnerFrames[0], reset)
		}
		mu.Unlock()
		for {
			select {
			case <-spinnerDone:
				return
			case <-ticker.C:
				mu.Lock()
				if !firstToken {
					i = (i + 1) % len(spinnerFrames)
					fmt.Printf("\r  %s%c Generating...%s", dim, spinnerFrames[i], reset)
				}
				mu.Unlock()
			}
		}
	}()

	message, usage, err := sp.GenerateCommitMessageStream(ctx, func(token string) {
		mu.Lock()
		defer mu.Unlock()
		if !firstToken {
			firstToken = true
			fmt.Printf("\r\033[K") // clear spinner line
			fmt.Printf("  %s%s› %s", bold, purple, token)
		} else {
			fmt.Print(token)
		}
	})

	close(spinnerDone)

	mu.Lock()
	if firstToken {
		fmt.Printf("%s\n", reset) // reset color after streamed message
	} else {
		fmt.Print("\r\033[K") // clear spinner if no tokens came
	}
	mu.Unlock()

	return message, usage, err
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

	fmt.Printf("  %s✓%s Key saved for %s.\n\n", green, reset, cfg.Provider)
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
		fmt.Printf("  › %s", string(line))
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

// colorizeDiffStat colorizes a single diff stat line.
// Lines like "file.go | 29 ++---" get colored +/-.
// Summary lines like "14 files changed, 895 insertions(+), 594 deletions(-)" get dim.
func colorizeDiffStat(line string) string {
	// Summary line (last line of diff stat)
	if strings.Contains(line, "files changed") || strings.Contains(line, "file changed") {
		return dim + line + reset
	}

	// Per-file lines: "file.go | 29 ++---"
	if m := diffStatRe.FindStringSubmatch(line); m != nil {
		prefix := m[1] // "file.go | 29 "
		plus := m[2]   // "+++"
		minus := m[3]  // "---"
		var b strings.Builder
		b.WriteString(prefix)
		if plus != "" {
			b.WriteString(green + plus + reset)
		}
		if minus != "" {
			b.WriteString(red + minus + reset)
		}
		return b.String()
	}

	return line
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
