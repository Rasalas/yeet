package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/evaldb"
	"github.com/spf13/cobra"
)

type evalSelectionOptions struct {
	Sample     int
	EditedOnly bool
	Contains   string
	Provider   string
	Model      string
	PromptFile string
	BatchSize  int
	MaxCostUSD float64
}

var (
	evalPlanOpts      evalSelectionOptions
	evalGenerateOpts  evalSelectionOptions
	evalJudgeVariant  int64
	evalReportVariant int64
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Offline A/B evaluation for commit message quality",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunAsCommit("eval", args)
		}
		return cmd.Help()
	},
}

var evalPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Estimate cost and sample size without making API calls",
	RunE:  runEvalPlan,
}

var evalGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate A/B candidates for a bounded subset of historical runs",
	RunE:  runEvalGenerate,
}

var evalJudgeCmd = &cobra.Command{
	Use:   "judge",
	Short: "Blind-judge generated A/B candidates",
	RunE:  runEvalJudge,
}

var evalReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Summarize wins, ties, cost, latency, and phrase-hit stats",
	RunE:  runEvalReport,
}

func init() {
	addEvalSelectionFlags(evalPlanCmd, &evalPlanOpts)
	addEvalSelectionFlags(evalGenerateCmd, &evalGenerateOpts)

	evalJudgeCmd.Flags().Int64Var(&evalJudgeVariant, "variant", 0, "Eval variant ID (default: latest)")
	evalReportCmd.Flags().Int64Var(&evalReportVariant, "variant", 0, "Eval variant ID (default: latest)")

	evalCmd.AddCommand(evalPlanCmd)
	evalCmd.AddCommand(evalGenerateCmd)
	evalCmd.AddCommand(evalJudgeCmd)
	evalCmd.AddCommand(evalReportCmd)
	rootCmd.AddCommand(evalCmd)
}

func addEvalSelectionFlags(cmd *cobra.Command, opts *evalSelectionOptions) {
	cmd.Flags().IntVar(&opts.Sample, "sample", 20, "Maximum number of historical runs to consider")
	cmd.Flags().BoolVar(&opts.EditedOnly, "edited-only", false, "Use only runs where the AI message was edited before commit")
	cmd.Flags().StringVar(&opts.Contains, "contains", "", "Filter runs where the baseline AI message contains this phrase")
	cmd.Flags().StringVar(&opts.Provider, "provider", "", "Provider for candidate generation (default: current config)")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Model override for candidate generation")
	cmd.Flags().StringVar(&opts.PromptFile, "prompt-file", "", "Prompt file for candidate generation (default: current prompt)")
	cmd.Flags().IntVar(&opts.BatchSize, "batch-size", 5, "Maximum number of candidates to generate")
	cmd.Flags().Float64Var(&opts.MaxCostUSD, "max-cost-usd", 1.0, "Upper bound for estimated generation cost in USD")
}

func runEvalPlan(cmd *cobra.Command, args []string) error {
	store, err := evaldb.Open()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	targetCfg, providerName, modelName, err := resolveEvalTarget(cfg, evalPlanOpts.Provider, evalPlanOpts.Model)
	if err != nil {
		return err
	}
	_ = targetCfg // kept for parity with generate path

	_, promptHash, promptPath, err := loadPrompt(evalPlanOpts.PromptFile)
	if err != nil {
		return err
	}

	runs, err := store.SelectEligibleRuns(evalPlanOpts.Sample, evalPlanOpts.EditedOnly, evalPlanOpts.Contains)
	if err != nil {
		return err
	}

	selected, estCost, unknownSkipped, budgetSkipped := selectRunsForBudget(runs, modelName, evalPlanOpts.BatchSize, evalPlanOpts.MaxCostUSD)

	fmt.Println()
	fmt.Println("  Eval plan")
	fmt.Printf("  DB: %s\n", store.Path())
	fmt.Printf("  Target: provider=%s model=%s\n", providerName, modelName)
	fmt.Printf("  Prompt: %s (sha256:%s)\n", promptPath, shortHash(promptHash))
	fmt.Printf("  Eligible runs: %d\n", len(runs))
	fmt.Printf("  Selected for generation: %d (batch-size=%d)\n", len(selected), evalPlanOpts.BatchSize)
	if evalPlanOpts.MaxCostUSD > 0 {
		fmt.Printf("  Estimated cost: $%.4f (cap: $%.4f)\n", estCost, evalPlanOpts.MaxCostUSD)
	} else {
		fmt.Printf("  Estimated cost: $%.4f\n", estCost)
	}
	if unknownSkipped > 0 {
		fmt.Printf("  Skipped (unknown pricing): %d\n", unknownSkipped)
	}
	if budgetSkipped > 0 {
		fmt.Printf("  Skipped (budget): %d\n", budgetSkipped)
	}
	fmt.Println()

	return nil
}

func runEvalGenerate(cmd *cobra.Command, args []string) error {
	store, err := evaldb.Open()
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
	}

	targetCfg, providerName, modelName, err := resolveEvalTarget(cfg, evalGenerateOpts.Provider, evalGenerateOpts.Model)
	if err != nil {
		return err
	}

	provider, err := ai.NewProvider(targetCfg)
	if err != nil {
		return err
	}

	promptText, promptHash, promptPath, err := loadPrompt(evalGenerateOpts.PromptFile)
	if err != nil {
		return err
	}

	runs, err := store.SelectEligibleRuns(evalGenerateOpts.Sample, evalGenerateOpts.EditedOnly, evalGenerateOpts.Contains)
	if err != nil {
		return err
	}

	selected, estCost, unknownSkipped, budgetSkipped := selectRunsForBudget(runs, modelName, evalGenerateOpts.BatchSize, evalGenerateOpts.MaxCostUSD)
	if len(selected) == 0 {
		fmt.Println()
		fmt.Println("  No runs selected for generation.")
		fmt.Println("  tip: increase --sample or --max-cost-usd, or relax filters.")
		fmt.Println()
		return nil
	}

	variantID, err := store.CreateVariant(evaldb.VariantSpec{
		Provider:    providerName,
		Model:       modelName,
		PromptHash:  promptHash,
		PromptText:  promptText,
		PromptPath:  promptPath,
		SampleLimit: evalGenerateOpts.Sample,
		BatchSize:   evalGenerateOpts.BatchSize,
		MaxCostUSD:  evalGenerateOpts.MaxCostUSD,
		EditedOnly:  evalGenerateOpts.EditedOnly,
		Contains:    evalGenerateOpts.Contains,
	})
	if err != nil {
		return err
	}

	var successes int
	var failures int
	var spent float64
	for i, run := range selected {
		fmt.Printf("  [%d/%d] run #%d\n", i+1, len(selected), run.ID)

		ctx := ai.CommitContext{
			Diff:          run.Diff,
			Branch:        run.Branch,
			RecentCommits: run.RecentCommits,
			Status:        run.Status,
			SystemPrompt:  promptText,
		}

		start := time.Now()
		msg, usage, genErr := provider.GenerateCommitMessage(ctx)
		latencyMs := time.Since(start).Milliseconds()

		record := evaldb.CandidateRecord{
			VariantID: variantID,
			RunID:     run.ID,
			LatencyMS: latencyMs,
		}

		if genErr != nil {
			record.Error = genErr.Error()
			failures++
		} else {
			costUSD, _ := usage.CostUSD()
			record.Message = msg
			record.InputTokens = usage.InputTokens
			record.OutputTokens = usage.OutputTokens
			record.CostUSD = costUSD
			spent += costUSD
			successes++
		}

		if err := store.InsertCandidate(record); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Printf("  Variant #%d created\n", variantID)
	fmt.Printf("  Target: provider=%s model=%s\n", providerName, modelName)
	fmt.Printf("  Prompt: %s (sha256:%s)\n", promptPath, shortHash(promptHash))
	fmt.Printf("  Planned estimate: $%.4f\n", estCost)
	fmt.Printf("  Actual generated: %d ok, %d failed, $%.4f spent\n", successes, failures, spent)
	if unknownSkipped > 0 {
		fmt.Printf("  Skipped (unknown pricing): %d\n", unknownSkipped)
	}
	if budgetSkipped > 0 {
		fmt.Printf("  Skipped (budget): %d\n", budgetSkipped)
	}
	fmt.Println()
	fmt.Printf("  Next: yeet eval judge --variant %d\n", variantID)
	fmt.Printf("        yeet eval report --variant %d\n", variantID)
	fmt.Println()

	return nil
}

func runEvalJudge(cmd *cobra.Command, args []string) error {
	store, err := evaldb.Open()
	if err != nil {
		return err
	}

	variant, ok, err := resolveVariant(store, evalJudgeVariant)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println()
		fmt.Println("  No eval variants found. Run `yeet eval generate` first.")
		fmt.Println()
		return nil
	}

	pending, err := store.PendingCount(variant.ID)
	if err != nil {
		return err
	}
	if pending == 0 {
		fmt.Println()
		fmt.Printf("  Variant #%d has no pending items to judge.\n\n", variant.ID)
		return nil
	}

	fmt.Println()
	fmt.Printf("  Judging variant #%d (%d pending)\n", variant.ID, pending)
	fmt.Println("  keys: a = option A, b = option B, t = tie, x = both bad, q = quit")
	fmt.Println()

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	reader := bufio.NewReader(os.Stdin)

	judgedNow := 0
	for {
		item, hasNext, err := store.NextUnvotedCandidate(variant.ID)
		if err != nil {
			return err
		}
		if !hasNext {
			break
		}

		leftSource := "baseline"
		rightSource := "candidate"
		leftText := strings.TrimSpace(item.BaselineMessage)
		rightText := strings.TrimSpace(item.CandidateMessage)
		if rng.Intn(2) == 1 {
			leftSource, rightSource = rightSource, leftSource
			leftText, rightText = rightText, leftText
		}

		fmt.Printf("  Run #%d (%s)\n", item.RunID, item.RunCreatedAt)
		fmt.Println("  Option A")
		fmt.Println("  --------")
		fmt.Println("  " + leftText)
		fmt.Println()
		fmt.Println("  Option B")
		fmt.Println("  --------")
		fmt.Println("  " + rightText)
		fmt.Println()

		choice, err := readJudgeChoice(reader)
		if err != nil {
			return err
		}
		if choice == "q" {
			fmt.Println()
			fmt.Printf("  Stopped. Judged %d item(s) in this session.\n\n", judgedNow)
			return nil
		}

		winner := "tie"
		switch choice {
		case "a":
			winner = leftSource
		case "b":
			winner = rightSource
		case "x":
			winner = "both_bad"
		}

		if err := store.SaveVote(item.CandidateID, winner, leftSource, rightSource); err != nil {
			return err
		}
		judgedNow++
		fmt.Println()
	}

	fmt.Printf("  Done. Judged %d item(s).\n\n", judgedNow)
	return nil
}

func runEvalReport(cmd *cobra.Command, args []string) error {
	store, err := evaldb.Open()
	if err != nil {
		return err
	}

	variant, ok, err := resolveVariant(store, evalReportVariant)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println()
		fmt.Println("  No eval variants found.")
		fmt.Println()
		return nil
	}

	report, err := store.VariantReport(variant.ID)
	if err != nil {
		return err
	}

	phrase := strings.TrimSpace(variant.Contains)
	phraseStats := evaldb.PhraseStats{}
	if phrase != "" {
		phraseStats, err = store.PhraseReport(variant.ID, phrase)
		if err != nil {
			return err
		}
	}

	decisive := report.CandidateWins + report.BaselineWins
	winRate := 0.0
	if decisive > 0 {
		winRate = float64(report.CandidateWins) / float64(decisive) * 100
	}

	fmt.Println()
	fmt.Printf("  Eval report: variant #%d\n", variant.ID)
	fmt.Printf("  Created: %s\n", variant.CreatedAt)
	fmt.Printf("  Target: provider=%s model=%s\n", variant.Provider, variant.Model)
	fmt.Printf("  Prompt: %s (sha256:%s)\n", variant.PromptPath, shortHash(variant.PromptHash))
	fmt.Println()
	fmt.Printf("  Candidates: %d total, %d generated, %d failed, %d judged\n",
		report.TotalCandidates, report.GeneratedCandidates, report.FailedCandidates, report.JudgedCandidates)
	fmt.Printf("  Outcomes: candidate=%d baseline=%d tie=%d both_bad=%d\n",
		report.CandidateWins, report.BaselineWins, report.Ties, report.BothBad)
	if decisive > 0 {
		fmt.Printf("  Candidate win-rate (decisive only): %.1f%%\n", winRate)
	}
	fmt.Printf("  Avg cost: candidate=$%.4f baseline=$%.4f\n",
		report.CandidateAvgCost, report.BaselineAvgCost)
	fmt.Printf("  Avg latency: candidate=%.0fms baseline=%.0fms\n",
		report.CandidateAvgLatency, report.BaselineAvgLatency)

	if phrase != "" {
		fmt.Println()
		fmt.Printf("  Phrase check (%q): baseline=%d candidate=%d (n=%d)\n",
			phrase, phraseStats.BaselineHits, phraseStats.CandidateHits, phraseStats.Compared)
	}

	fmt.Println()
	return nil
}

func resolveEvalTarget(cfg config.Config, providerFlag, modelFlag string) (config.Config, string, string, error) {
	out := cfg

	providerName := strings.TrimSpace(providerFlag)
	if providerName == "" {
		providerName = out.Provider
	}
	out.Provider = providerName

	if modelFlag != "" {
		if providerName == "auto" {
			return config.Config{}, "", "", fmt.Errorf("--model requires an explicit --provider (not auto)")
		}
		out.SetModel(providerName, modelFlag)
	}

	if providerName == "auto" {
		modelName := ai.AutoModelName(out)
		if modelName == "" {
			return config.Config{}, "", "", fmt.Errorf("auto provider has no available model (missing API keys)")
		}
		return out, providerName, modelName, nil
	}

	rp, ok := out.ResolveProviderFull(providerName)
	if !ok {
		return config.Config{}, "", "", fmt.Errorf("unknown provider: %s", providerName)
	}
	if rp.Model == "" {
		return config.Config{}, "", "", fmt.Errorf("provider %s has no model configured", providerName)
	}
	return out, providerName, rp.Model, nil
}

func loadPrompt(promptFile string) (text, hash, source string, err error) {
	if strings.TrimSpace(promptFile) == "" {
		text = ai.LoadPrompt()
		path, pathErr := ai.PromptPath()
		if pathErr != nil {
			source = "<default>"
		} else {
			source = path
		}
		return text, hashText(text), source, nil
	}

	data, readErr := os.ReadFile(promptFile)
	if readErr != nil {
		return "", "", "", readErr
	}
	text = strings.TrimSpace(string(data))
	if text == "" {
		return "", "", "", fmt.Errorf("prompt file is empty: %s", promptFile)
	}
	return text, hashText(text), promptFile, nil
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func shortHash(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func selectRunsForBudget(runs []evaldb.Run, model string, batchSize int, maxCostUSD float64) ([]evaldb.Run, float64, int, int) {
	if batchSize <= 0 {
		batchSize = 5
	}

	selected := make([]evaldb.Run, 0, min(len(runs), batchSize))
	totalEstimate := 0.0
	unknownSkipped := 0
	budgetSkipped := 0

	for _, run := range runs {
		if len(selected) >= batchSize {
			break
		}

		est, known := ai.EstimateCost(model, run.InputTokens, run.OutputTokens)
		if maxCostUSD > 0 {
			// Unknown pricing means no safe budget check.
			if !known {
				unknownSkipped++
				continue
			}
			if totalEstimate+est > maxCostUSD {
				budgetSkipped++
				continue
			}
		}

		selected = append(selected, run)
		if known {
			totalEstimate += est
		}
	}

	return selected, totalEstimate, unknownSkipped, budgetSkipped
}

func resolveVariant(store *evaldb.Store, requestedID int64) (evaldb.Variant, bool, error) {
	if requestedID > 0 {
		return store.GetVariant(requestedID)
	}
	return store.LatestVariant()
}

func readJudgeChoice(reader *bufio.Reader) (string, error) {
	for {
		fmt.Print("  Choose [a/b/t/x/q]: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		choice := strings.ToLower(strings.TrimSpace(line))
		switch choice {
		case "a", "b", "t", "x", "q":
			return choice, nil
		}
		fmt.Println("  Invalid input. Use a, b, t, x, or q.")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
