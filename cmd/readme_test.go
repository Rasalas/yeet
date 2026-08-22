package cmd

import (
	"os"
	"regexp"
	"testing"
)

// TestReadmeDocumentsAllSubcommands keeps the README command table in sync
// with the registered root subcommands: every visible subcommand must be
// mentioned as `yeet <name>` in the README.
func TestReadmeDocumentsAllSubcommands(t *testing.T) {
	raw, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatalf("read README: %v", err)
	}

	documented := map[string]bool{}
	re := regexp.MustCompile("`yeet ([a-z]+)")
	for _, match := range re.FindAllStringSubmatch(string(raw), -1) {
		documented[match[1]] = true
	}

	skip := map[string]bool{
		"help":       true, // auto-added by cobra
		"completion": true, // auto-added by cobra
	}

	for _, sub := range rootCmd.Commands() {
		name := sub.Name()
		if name == "" || skip[name] {
			continue
		}
		if !documented[name] {
			t.Errorf("subcommand %q is missing from the README (add a `yeet %s` row or section)", name, name)
		}
	}
}
