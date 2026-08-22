package cmd

import (
	"strings"
	"testing"

	"github.com/rasalas/yeet/internal/keyring"
)

func keyringStatusFixture() map[string]keyring.KeyInfo {
	return map[string]keyring.KeyInfo{
		"anthropic": {Found: true, Source: keyring.SourceKeyring},
		"openai":    {Found: true, Source: keyring.SourceEnv}, // not removable via reset
		"groq":      {Found: false},
	}
}

func overrideKeyringSeams(t *testing.T, status map[string]keyring.KeyInfo) *[]string {
	t.Helper()

	origLookup, origRemove, origConfirm, origYes := keyringLookup, keyringRemove, confirmReset, yesFlag
	t.Cleanup(func() {
		keyringLookup, keyringRemove, confirmReset, yesFlag = origLookup, origRemove, origConfirm, origYes
	})

	keyringLookup = func(providers []string, envs map[string]string) map[string]keyring.KeyInfo {
		return status
	}
	var removed []string
	keyringRemove = func(provider string) error {
		removed = append(removed, provider)
		return nil
	}
	return &removed
}

func TestRunAuthResetWithoutKeysPromptsNothing(t *testing.T) {
	removed := overrideKeyringSeams(t, map[string]keyring.KeyInfo{})
	yesFlag = false

	if err := runAuthReset(nil, nil); err != nil {
		t.Fatalf("runAuthReset returned error: %v", err)
	}
	if len(*removed) != 0 {
		t.Errorf("removed = %v, want none", *removed)
	}
}

func TestRunAuthResetDeclinedRemovesNothing(t *testing.T) {
	removed := overrideKeyringSeams(t, keyringStatusFixture())
	yesFlag = false
	confirmReset = func() (bool, error) { return false, nil }

	if err := runAuthReset(nil, nil); err != nil {
		t.Fatalf("runAuthReset returned error: %v", err)
	}
	if len(*removed) != 0 {
		t.Errorf("removed = %v after decline, want none", *removed)
	}
}

func TestRunAuthResetConfirmedRemovesOnlyKeyringKeys(t *testing.T) {
	removed := overrideKeyringSeams(t, keyringStatusFixture())
	yesFlag = false
	confirmReset = func() (bool, error) { return true, nil }

	if err := runAuthReset(nil, nil); err != nil {
		t.Fatalf("runAuthReset returned error: %v", err)
	}
	want := []string{"anthropic"}
	if len(*removed) != 1 || (*removed)[0] != want[0] {
		t.Errorf("removed = %v, want %v", *removed, want)
	}
}

func TestRunAuthResetWithYesFlagSkipsPrompt(t *testing.T) {
	removed := overrideKeyringSeams(t, keyringStatusFixture())
	yesFlag = true
	prompted := false
	confirmReset = func() (bool, error) { prompted = true; return true, nil }

	if err := runAuthReset(nil, nil); err != nil {
		t.Fatalf("runAuthReset returned error: %v", err)
	}
	if prompted {
		t.Error("confirmation prompt shown despite --yes")
	}
	if len(*removed) == 0 || !strings.Contains(strings.Join(*removed, ","), "anthropic") {
		t.Errorf("removed = %v, want anthropic", *removed)
	}
}
