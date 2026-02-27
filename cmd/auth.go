package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
	goterm "golang.org/x/term"
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

	fmt.Printf("\n  %sAPI Keys%s\n\n", term.Bold, term.Reset)
	for _, p := range providers {
		info := status[p]
		if info.Found {
			source := string(info.Source)
			line := fmt.Sprintf("  %s\u2713%s  %-16s%s%s%s", term.Green, term.Reset, p, term.Dim, source, term.Reset)
			if info.Source != keyring.SourceKeyring {
				line += fmt.Sprintf("  %s\u2190 yeet auth import %s%s", term.Dim, p, term.Reset)
			}
			fmt.Println(line)
		} else {
			fmt.Printf("  %s\u2717%s  %s\n", term.Red, term.Reset, p)
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
	key, err := goterm.ReadPassword(int(os.Stdin.Fd()))
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

	fmt.Printf("  %s\u2713%s API key for %s saved to keyring.\n", term.Green, term.Reset, provider)
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

	fmt.Printf("  %s\u2713%s API key for %s removed from keyring.\n", term.Green, term.Reset, provider)
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
				fmt.Printf("  %s\u2717%s %s: no key found to import\n", term.Red, term.Reset, p)
			}
			continue
		}
		if source == keyring.SourceKeyring {
			if len(args) == 1 {
				fmt.Printf("  %s\u00b7%s %s: already in keyring\n", term.Dim, term.Reset, p)
			}
			continue
		}
		if err := keyring.Set(p, key); err != nil {
			fmt.Printf("  %s\u2717%s %s: failed to import: %v\n", term.Red, term.Reset, p, err)
			continue
		}
		fmt.Printf("  %s\u2713%s %s: imported from %s to keyring\n", term.Green, term.Reset, p, source)
		imported++
	}

	if len(args) == 0 && imported == 0 {
		fmt.Printf("  %sNothing to import.%s\n", term.Dim, term.Reset)
	}

	return nil
}

func runAuthReset(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load()
	providers := cfg.AllProviders()
	deleted := 0
	for _, p := range providers {
		if err := keyring.Delete(p); err == nil {
			fmt.Printf("  %s\u2713%s %s removed\n", term.Green, term.Reset, p)
			deleted++
		}
	}
	if deleted == 0 {
		fmt.Printf("  %sNo keys in keyring.%s\n", term.Dim, term.Reset)
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
