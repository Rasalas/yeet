package cmd

import (
	"testing"

	"github.com/rasalas/yeet/internal/evaldb"
)

func TestSelectRunsForBudget(t *testing.T) {
	runs := []evaldb.Run{
		{ID: 1, InputTokens: 1000, OutputTokens: 100}, // known cost
		{ID: 2, InputTokens: 2000, OutputTokens: 200}, // known cost
		{ID: 3, InputTokens: 3000, OutputTokens: 300}, // known cost
	}

	selected, estCost, unknownSkipped, budgetSkipped := selectRunsForBudget(runs, "gpt-4o-mini", 2, 1.0)
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if estCost <= 0 {
		t.Fatalf("estCost = %f, want > 0", estCost)
	}
	if unknownSkipped != 0 {
		t.Fatalf("unknownSkipped = %d, want 0", unknownSkipped)
	}
	if budgetSkipped != 0 {
		t.Fatalf("budgetSkipped = %d, want 0", budgetSkipped)
	}
}

func TestSelectRunsForBudgetSkipsUnknownPricing(t *testing.T) {
	runs := []evaldb.Run{
		{ID: 1, InputTokens: 1000, OutputTokens: 100},
	}

	selected, _, unknownSkipped, budgetSkipped := selectRunsForBudget(runs, "unknown/model", 5, 1.0)
	if len(selected) != 0 {
		t.Fatalf("selected count = %d, want 0", len(selected))
	}
	if unknownSkipped != 1 {
		t.Fatalf("unknownSkipped = %d, want 1", unknownSkipped)
	}
	if budgetSkipped != 0 {
		t.Fatalf("budgetSkipped = %d, want 0", budgetSkipped)
	}
}
