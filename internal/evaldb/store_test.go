package evaldb

import "testing"

func TestInsertAndSelectEligibleRuns(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	err = store.InsertRun(RunRecord{
		RepoPath:      "/tmp/repo",
		Command:       "commit",
		Branch:        "feat/test",
		Status:        "M file.go",
		RecentCommits: "abc123 test",
		Diff:          "diff --git a/file.go b/file.go\n+hello",
		Provider:      "openai",
		Model:         "gpt-4o-mini",
		PromptHash:    "hash",
		PromptText:    "prompt",
		AIMessage:     "fix(core): update parser",
		FinalMessage:  "fix(core): update parser",
		UserAction:    "accepted",
		InputTokens:   1000,
		OutputTokens:  50,
		CostUSD:       0.0002,
		LatencyMS:     120,
		LocalOnly:     false,
	})
	if err != nil {
		t.Fatalf("InsertRun() error = %v", err)
	}

	all, err := store.SelectEligibleRuns(10, false, "")
	if err != nil {
		t.Fatalf("SelectEligibleRuns() error = %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("len(all) = %d, want 1", len(all))
	}

	editedOnly, err := store.SelectEligibleRuns(10, true, "")
	if err != nil {
		t.Fatalf("SelectEligibleRuns(edited) error = %v", err)
	}
	if len(editedOnly) != 0 {
		t.Fatalf("len(editedOnly) = %d, want 0", len(editedOnly))
	}

	withPhrase, err := store.SelectEligibleRuns(10, false, "update parser")
	if err != nil {
		t.Fatalf("SelectEligibleRuns(contains) error = %v", err)
	}
	if len(withPhrase) != 1 {
		t.Fatalf("len(withPhrase) = %d, want 1", len(withPhrase))
	}
}

func TestVariantAndVotingFlow(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	store, err := Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if err := store.InsertRun(RunRecord{
		RepoPath:      "/tmp/repo",
		Command:       "commit",
		Branch:        "feat/test",
		Status:        "M file.go",
		RecentCommits: "abc123 test",
		Diff:          "diff --git a/file.go b/file.go\n+hello",
		Provider:      "openai",
		Model:         "gpt-4o-mini",
		PromptHash:    "hash",
		PromptText:    "prompt",
		AIMessage:     "fix(core): old message",
		FinalMessage:  "fix(core): old message",
		UserAction:    "accepted",
		InputTokens:   1000,
		OutputTokens:  50,
		CostUSD:       0.0002,
		LatencyMS:     100,
		LocalOnly:     false,
	}); err != nil {
		t.Fatalf("InsertRun() error = %v", err)
	}

	runs, err := store.SelectEligibleRuns(1, false, "")
	if err != nil {
		t.Fatalf("SelectEligibleRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}

	variantID, err := store.CreateVariant(VariantSpec{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		PromptHash:  "newhash",
		PromptText:  "new prompt",
		PromptPath:  "/tmp/prompt.txt",
		SampleLimit: 20,
		BatchSize:   5,
		MaxCostUSD:  1.0,
	})
	if err != nil {
		t.Fatalf("CreateVariant() error = %v", err)
	}

	if err := store.InsertCandidate(CandidateRecord{
		VariantID:    variantID,
		RunID:        runs[0].ID,
		Message:      "fix(core): new message",
		InputTokens:  900,
		OutputTokens: 40,
		CostUSD:      0.00018,
		LatencyMS:    80,
	}); err != nil {
		t.Fatalf("InsertCandidate() error = %v", err)
	}

	pending, err := store.PendingCount(variantID)
	if err != nil {
		t.Fatalf("PendingCount() error = %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending = %d, want 1", pending)
	}

	item, ok, err := store.NextUnvotedCandidate(variantID)
	if err != nil {
		t.Fatalf("NextUnvotedCandidate() error = %v", err)
	}
	if !ok {
		t.Fatal("NextUnvotedCandidate() ok = false, want true")
	}

	if err := store.SaveVote(item.CandidateID, "candidate", "baseline", "candidate"); err != nil {
		t.Fatalf("SaveVote() error = %v", err)
	}

	pending, err = store.PendingCount(variantID)
	if err != nil {
		t.Fatalf("PendingCount() error = %v", err)
	}
	if pending != 0 {
		t.Fatalf("pending = %d, want 0", pending)
	}

	report, err := store.VariantReport(variantID)
	if err != nil {
		t.Fatalf("VariantReport() error = %v", err)
	}
	if report.CandidateWins != 1 {
		t.Fatalf("report.CandidateWins = %d, want 1", report.CandidateWins)
	}
}
