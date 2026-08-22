package cmd

import "testing"

func TestSetVersionWiresRootAndACPClients(t *testing.T) {
	origVersion := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = origVersion })

	SetVersion("v9.9.9")
	if rootCmd.Version != "v9.9.9" {
		t.Errorf("rootCmd.Version = %q, want %q", rootCmd.Version, "v9.9.9")
	}
}

func TestSetVersionIgnoresEmpty(t *testing.T) {
	origVersion := rootCmd.Version
	t.Cleanup(func() { rootCmd.Version = origVersion })

	rootCmd.Version = "keep"
	SetVersion("")
	if rootCmd.Version != "keep" {
		t.Errorf("rootCmd.Version = %q, want %q", rootCmd.Version, "keep")
	}
}
