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

var authImportCmd = &cobra.Command{
	Use:   "import [provider]",
	Short: "Import keys from env/opencode into OS keyring",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runAuthImport,
}

var authResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove all API keys from the OS keyring",
	RunE:  runAuthReset,
}

func init() {
	authCmd.AddCommand(authSetCmd)
	authCmd.AddCommand(authDeleteCmd)
	authCmd.AddCommand(authImportCmd)
	authCmd.AddCommand(authResetCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	providers := cfg.AllProviders()
	status := keyring.Status(providers, cfg.CustomEnvs())

	fmt.Println("\n  API Key Status:")
	fmt.Println()
	for _, p := range providers {
		info := status[p]
		if info.Found {
			source := string(info.Source)
			line := fmt.Sprintf("  ✓  %-16s(%s)", p, source)
			if info.Source != keyring.SourceKeyring {
				line += fmt.Sprintf("  ← yeet auth import %s", p)
			}
			fmt.Println(line)
		} else {
			fmt.Printf("  ✗  %s\n", p)
		}
	}
	fmt.Println()
	return nil
}

func runAuthSet(cmd *cobra.Command, args []string) error {
	provider := strings.ToLower(args[0])
	if !isValidProvider(provider) {
		return fmt.Errorf("unknown provider: %s (valid: %s)", provider, strings.Join(allProviders(), ", "))
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
		return fmt.Errorf("unknown provider: %s (valid: %s)", provider, strings.Join(allProviders(), ", "))
	}

	if err := keyring.Delete(provider); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	fmt.Printf("  ✓ API key for %s removed from keyring.\n", provider)
	return nil
}

func runAuthImport(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	envs := cfg.CustomEnvs()

	var targets []string
	if len(args) == 1 {
		p := strings.ToLower(args[0])
		if !isValidProvider(p) {
			return fmt.Errorf("unknown provider: %s (valid: %s)", p, strings.Join(allProviders(), ", "))
		}
		targets = []string{p}
	} else {
		targets = cfg.AllProviders()
	}

	imported := 0
	for _, p := range targets {
		key, source := keyring.Resolve(p, envs[p])
		if key == "" {
			if len(args) == 1 {
				fmt.Printf("  ✗ %s: no key found to import\n", p)
			}
			continue
		}
		if source == keyring.SourceKeyring {
			if len(args) == 1 {
				fmt.Printf("  · %s: already in keyring\n", p)
			}
			continue
		}
		if err := keyring.Set(p, key); err != nil {
			fmt.Printf("  ✗ %s: failed to import: %v\n", p, err)
			continue
		}
		fmt.Printf("  ✓ %s: imported from %s to keyring\n", p, source)
		imported++
	}

	if len(args) == 0 && imported == 0 {
		fmt.Println("  Nothing to import.")
	}

	return nil
}

func runAuthReset(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	providers := cfg.AllProviders()
	deleted := 0
	for _, p := range providers {
		if err := keyring.Delete(p); err == nil {
			fmt.Printf("  ✓ %s removed\n", p)
			deleted++
		}
	}
	if deleted == 0 {
		fmt.Println("  No keys in keyring.")
	}
	return nil
}

func allProviders() []string {
	cfg, _ := config.Load()
	return cfg.AllProviders()
}

func isValidProvider(p string) bool {
	for _, v := range allProviders() {
		if v == p {
			return true
		}
	}
	return false
}
