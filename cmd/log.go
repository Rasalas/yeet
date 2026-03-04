package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/scanner"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
)

var (
	logSince  string
	logUntil  string
	logAuthor string
	logExport string
)

func init() {
	logCmd.Flags().StringVar(&logSince, "since", "", "Start date (default: 14 days ago)")
	logCmd.Flags().StringVar(&logUntil, "until", "", "End date (default: now)")
	logCmd.Flags().StringVar(&logAuthor, "author", "", "Filter by author (default: git user.name)")
	logCmd.Flags().StringVar(&logExport, "export", "", "Export to file (.md or .json)")
	rootCmd.AddCommand(logCmd)
}

var logCmd = &cobra.Command{
	Use:   "log <path>...",
	Short: "AI-powered work recap across multiple repos",
	Long:  "Scan git history across project directories, summarize commits into clean recap cards via AI.",
	Args:  cobra.MinimumNArgs(1),

	SilenceUsage: true,
	RunE:         runLog,
}

func runLog(cmd *cobra.Command, args []string) error {
	// Defaults
	if logSince == "" {
		logSince = time.Now().AddDate(0, 0, -14).Format("2006-01-02")
	}
	if logUntil == "" {
		logUntil = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	}
	if logAuthor == "" {
		logAuthor = scanner.DefaultAuthor()
	}

	// 1. Find repos
	repos, err := scanner.FindRepos(args)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		fmt.Printf("\n  %sNo git repos found in the given paths.%s\n\n", term.Dim, term.Reset)
		return nil
	}

	fmt.Printf("\n  %sFound %d repo(s), scanning commits by %s since %s...%s\n",
		term.Dim, len(repos), logAuthor, logSince, term.Reset)

	// 2. Collect logs
	logs, err := scanner.CollectLogs(repos, logSince, logUntil, logAuthor)
	if err != nil {
		return err
	}
	if len(logs) == 0 {
		fmt.Printf("\n  %sNo commits found in the given date range.%s\n\n", term.Dim, term.Reset)
		return nil
	}

	fmt.Printf("  %s%d repo(s) with commits%s\n", term.Dim, len(logs), term.Reset)

	// 3. Build AI prompt
	var userMsg strings.Builder
	for _, rl := range logs {
		fmt.Fprintf(&userMsg, "## %s\n\n", rl.Name)
		fmt.Fprintf(&userMsg, "### Commits\n%s\n\n", rl.Log)
		if rl.Stat != "" {
			fmt.Fprintf(&userMsg, "### Stats\n%s\n\n", rl.Stat)
		}
	}

	// 4. AI generation
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	for model, p := range cfg.Pricing {
		ai.SetPricing(model, p.Input, p.Output)
	}

	provider, err := ai.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("no AI provider configured: %w", err)
	}

	ctx := ai.CommitContext{
		Diff:         userMsg.String(),
		SystemPrompt: ai.LogPrompt,
		MaxTokens:    4096,
	}

	var s term.Spinner
	s.Start("Generating recap...")

	msg, usage, genErr := provider.GenerateCommitMessage(ctx)
	s.Stop()

	if genErr != nil {
		return fmt.Errorf("AI generation failed: %w", genErr)
	}

	// 5. Parse cards
	cards := parseCards(msg)
	if len(cards) == 0 {
		fmt.Printf("\n  %sAI returned no cards.%s\n\n", term.Dim, term.Reset)
		return nil
	}

	// 6. Display
	fmt.Println()
	for _, c := range cards {
		term.DisplayCard(c.Title, c.Body)
	}

	// 7. Export
	if logExport != "" {
		if err := exportCards(cards, logExport); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Printf("  %s✓%s exported to %s\n", term.Green, term.Reset, logExport)
	}

	// 8. Usage
	if usage.InputTokens > 0 {
		costLine := fmt.Sprintf("%s · %s", usage.FormatTokens(), usage.Model)
		if cost, ok := usage.Cost(); ok {
			costLine = fmt.Sprintf("%s · %s · %s", cost, usage.FormatTokens(), usage.Model)
		}
		fmt.Printf("\n  %s%s%s\n", term.Dim, costLine, term.Reset)
	}
	fmt.Println()

	return nil
}

type card struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// parseCards splits the AI response into cards separated by "---".
func parseCards(raw string) []card {
	raw = strings.TrimSpace(raw)
	sections := strings.Split(raw, "\n---\n")

	var cards []card
	for _, sec := range sections {
		sec = strings.TrimSpace(sec)
		if sec == "" {
			continue
		}

		lines := strings.SplitN(sec, "\n", 2)
		title := strings.TrimSpace(lines[0])
		title = strings.TrimPrefix(title, "# ")

		var body string
		if len(lines) > 1 {
			body = strings.TrimSpace(lines[1])
		}

		cards = append(cards, card{Title: title, Body: body})
	}
	return cards
}

func exportCards(cards []card, path string) error {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		data, err := json.MarshalIndent(cards, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, append(data, '\n'), 0644)

	default: // .md and anything else
		var b strings.Builder
		for i, c := range cards {
			fmt.Fprintf(&b, "# %s\n\n%s\n", c.Title, c.Body)
			if i < len(cards)-1 {
				b.WriteString("\n---\n\n")
			}
		}
		return os.WriteFile(path, []byte(b.String()+"\n"), 0644)
	}
}
