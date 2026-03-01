package cmd

import (
	"os"
	"time"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/evaldb"
)

type commitRunCapture struct {
	Context   ai.CommitContext
	Prompt    string
	Provider  string
	Suggested string
	LatencyMS int64
}

func saveCommitRunCapture(c commitRunCapture, usage *ai.Usage, finalMessage, userAction string, localOnly bool) error {
	if usage == nil {
		return nil
	}

	repoPath, err := os.Getwd()
	if err != nil {
		repoPath = ""
	}

	costUSD, _ := usage.CostUSD()

	store, err := evaldb.Open()
	if err != nil {
		return err
	}

	return store.InsertRun(evaldb.RunRecord{
		CreatedAt:     time.Now().UTC(),
		RepoPath:      repoPath,
		Command:       "commit",
		Branch:        c.Context.Branch,
		Status:        c.Context.Status,
		RecentCommits: c.Context.RecentCommits,
		Diff:          c.Context.Diff,
		Provider:      c.Provider,
		Model:         usage.Model,
		PromptHash:    hashText(c.Prompt),
		PromptText:    c.Prompt,
		AIMessage:     c.Suggested,
		FinalMessage:  finalMessage,
		UserAction:    userAction,
		InputTokens:   usage.InputTokens,
		OutputTokens:  usage.OutputTokens,
		CostUSD:       costUSD,
		LatencyMS:     c.LatencyMS,
		LocalOnly:     localOnly,
	})
}
