package cmd

import (
	"fmt"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "doctor",
		Short: "Check configuration and provider status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return RunAsCommit("doctor", args)
			}
			return runDoctor()
		},
	})

	// Keep "log" as a hidden alias pointing to doctor.
	rootCmd.AddCommand(&cobra.Command{
		Use:    "log",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return RunAsCommit("log", args)
			}
			fmt.Printf("  %s\"yeet log\" is now \"yeet doctor\".%s\n\n", term.Dim, term.Reset)
			return runDoctor()
		},
	})
}

func runDoctor() error {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
		fmt.Printf("\n  %s! Could not load config: %v%s\n", term.Red, err, term.Reset)
	}

	// Active provider + model
	provider := cfg.Provider
	model := ""
	if provider == "auto" {
		model = ai.AutoModelName(cfg)
		if model == "" {
			model = "(no provider available)"
		}
	} else if rp, ok := cfg.ResolveProviderFull(provider); ok {
		model = rp.Model
	}

	fmt.Println()
	fmt.Printf("  %sProvider%s  %s\n", term.Bold, term.Reset, provider)
	fmt.Printf("  %sModel%s     %s\n", term.Bold, term.Reset, model)

	// Config path
	if path, err := config.Path(); err == nil {
		fmt.Printf("  %sConfig%s    %s%s%s\n", term.Bold, term.Reset, term.Dim, path, term.Reset)
	}

	// Validation
	problems := cfg.Validate()
	if len(problems) > 0 {
		fmt.Printf("\n  %sWarnings%s\n\n", term.Bold, term.Reset)
		for _, p := range problems {
			fmt.Printf("  %s!%s %s\n", term.Red, term.Reset, p)
		}
	}

	// Key status
	providers := cfg.AllProviders()
	envs := cfg.CustomEnvs()
	status := keyring.Status(providers, envs)

	fmt.Printf("\n  %sKeys%s\n\n", term.Bold, term.Reset)
	for _, p := range providers {
		entry, inRegistry := config.Registry[p]
		info := status[p]

		if !inRegistry || entry.NeedsAuth {
			if info.Found {
				fmt.Printf("  %s\u2713%s  %-16s%s%s%s\n", term.Green, term.Reset, p, term.Dim, info.Source, term.Reset)
			} else {
				hint := fmt.Sprintf("yeet auth set %s", p)
				if envName, ok := envs[p]; ok {
					hint = fmt.Sprintf("%s or %s", envName, hint)
				}
				fmt.Printf("  %s\u2717%s  %-16s%snot found  \u2190 %s%s\n", term.Red, term.Reset, p, term.Dim, hint, term.Reset)
			}
		} else {
			fmt.Printf("  %s\u00b7%s  %-16s%sno auth needed%s\n", term.Dim, term.Reset, p, term.Dim, term.Reset)
		}
	}

	// Summary
	fmt.Println()
	found := 0
	for _, info := range status {
		if info.Found {
			found++
		}
	}
	if found == 0 {
		fmt.Printf("  %sNo API keys configured. Run %syeet auth set <provider>%s to get started.%s\n", term.Dim, term.Reset+term.Bold, term.Reset+term.Dim, term.Reset)
	} else if len(problems) == 0 {
		fmt.Printf("  %s\u2713%s Everything looks good.\n", term.Green, term.Reset)
	} else {
		fmt.Printf("  %s%d warning(s) — see above.%s\n", term.Red, len(problems), term.Reset)
	}
	fmt.Println()

	return nil
}
