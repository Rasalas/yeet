package cmd

import (
	"fmt"
	"strings"
	"syscall"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage API keys in OS keyring",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunAsCommit("auth", args)
		}
		return runAuthStatus(cmd, args)
	},
}

var authSetCmd = &cobra.Command{
	Use:   "set <provider>",
	Short: "Store an API key in the OS keyring",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthSet,
}

var authDeleteCmd = &cobra.Command{
	Use:   "delete <provider>",
	Short: "Remove an API key from the OS keyring",
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthDelete,
}

func init() {
	authCmd.AddCommand(authSetCmd)
	authCmd.AddCommand(authDeleteCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	status := keyring.Status()
	fmt.Println("\n  API Key Status:")
	fmt.Println()
	for _, p := range config.Providers() {
		icon := "  ✗"
		if status[p] {
			icon = "  ✓"
		}
		fmt.Printf("  %s  %s\n", icon, p)
	}
	fmt.Println()
	return nil
}

func runAuthSet(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])
	if !isValidProvider(provider) {
		return fmt.Errorf("unknown provider: %s (valid: %s)", provider, strings.Join(config.Providers(), ", "))
	}

	fmt.Printf("  Enter API key for %s: ", provider)
	key, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read key: %w", err)
	}

	apiKey := strings.TrimSpace(string(key))
	if apiKey == "" {
		return fmt.Errorf("empty key, nothing saved")
	}

	if err := keyring.Set(provider, apiKey); err != nil {
		return fmt.Errorf("failed to save key: %w", err)
	}

	fmt.Printf("  ✓ API key for %s saved to keyring.\n", provider)
	return nil
}

func runAuthDelete(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])
	if !isValidProvider(provider) {
		return fmt.Errorf("unknown provider: %s (valid: %s)", provider, strings.Join(config.Providers(), ", "))
	}

	if err := keyring.Delete(provider); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	fmt.Printf("  ✓ API key for %s removed from keyring.\n", provider)
	return nil
}

func isValidProvider(p string) bool {
	for _, v := range config.Providers() {
		if v == p {
			return true
		}
	}
	return false
}
