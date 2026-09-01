package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/term"
	"github.com/spf13/cobra"
)

var doctorAIFlag bool

func init() {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check configuration and provider status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return RunAsCommit("doctor", args)
			}
			return runDoctor()
		},
	}
	doctorCmd.Flags().BoolVar(&doctorAIFlag, "ai", false, "Run a no-generation provider smoke test")
	rootCmd.AddCommand(doctorCmd)

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
		if (rp.Protocol == config.ProtocolACP || rp.Protocol == config.ProtocolPi) && model == "" {
			model = "(native CLI config)"
		}
	}

	fmt.Println()
	fmt.Printf("  %sProvider%s  %s\n", term.Bold, term.Reset, provider)
	if provider == "auto" {
		if autoProvider := ai.AutoProviderName(cfg); autoProvider != "" {
			fmt.Printf("  %sAuto%s      %s\n", term.Bold, term.Reset, autoProvider)
			if rp, ok := cfg.ResolveProviderFull(autoProvider); ok && (rp.Protocol == config.ProtocolACP || rp.Protocol == config.ProtocolPi) {
				fmt.Printf("  %sCommand%s   %s\n", term.Bold, term.Reset, ai.ProviderCommandLine(rp))
			}
		}
	}
	fmt.Printf("  %sModel%s     %s\n", term.Bold, term.Reset, model)
	if rp, ok := cfg.ResolveProviderFull(provider); ok && (rp.Protocol == config.ProtocolACP || rp.Protocol == config.ProtocolPi) {
		fmt.Printf("  %sCommand%s   %s\n", term.Bold, term.Reset, ai.ProviderCommandLine(rp))
	}

	// Config path
	if path, err := config.Path(); err == nil {
		fmt.Printf("  %sConfig%s    %s%s%s\n", term.Bold, term.Reset, term.Dim, path, term.Reset)
	}

	// Validation
	problems := cfg.Validate()
	if provider == "auto" && ai.AutoProviderName(cfg) == "" {
		problems = append(problems, "auto provider has no available provider from [auto].order")
	}
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
		info := status[p]
		rp, ok := cfg.ResolveProviderFull(p)

		if !ok || rp.NeedsAuth {
			if info.Found {
				fmt.Printf("  %s\u2713%s  %-16s%s%s%s\n", term.Green, term.Reset, p, term.Dim, info.Source, term.Reset)
			} else {
				hint := fmt.Sprintf("yeet auth set %s", p)
				if envName, ok := envs[p]; ok {
					hint = fmt.Sprintf("%s or %s", envName, hint)
				}
				fmt.Printf("  %s\u2717%s  %-16s%snot found  \u2190 %s%s\n", term.Red, term.Reset, p, term.Dim, hint, term.Reset)
			}
		} else if rp.Protocol == config.ProtocolACP || rp.Protocol == config.ProtocolPi {
			fmt.Printf("  %s\u00b7%s  %-16s%suses native CLI auth%s\n", term.Dim, term.Reset, p, term.Dim, term.Reset)
		} else {
			fmt.Printf("  %s\u00b7%s  %-16s%sno auth needed%s\n", term.Dim, term.Reset, p, term.Dim, term.Reset)
		}
	}

	smokeFailed := false
	if doctorAIFlag {
		fmt.Printf("\n  %sAI Smoke Test%s\n\n", term.Bold, term.Reset)
		label, err := runDoctorAISmoke(cfg)
		if err != nil {
			smokeFailed = true
			fmt.Printf("  %s✗%s  %-16s%s%s%s\n", term.Red, term.Reset, label, term.Dim, err, term.Reset)
		} else {
			fmt.Printf("  %s✓%s  %s%s%s\n", term.Green, term.Reset, term.Dim, label, term.Reset)
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
	activeReadyWithoutKey := false
	if provider != "auto" {
		if rp, ok := cfg.ResolveProviderFull(provider); ok && !rp.NeedsAuth {
			activeReadyWithoutKey = true
		}
	}
	warnings := len(problems)
	if smokeFailed {
		warnings++
	}
	if warnings == 0 && activeReadyWithoutKey {
		fmt.Printf("  %s\u2713%s Everything looks good.\n", term.Green, term.Reset)
	} else if found == 0 {
		fmt.Printf("  %sNo API keys configured. Run %syeet auth set <provider>%s to get started.%s\n", term.Dim, term.Reset+term.Bold, term.Reset+term.Dim, term.Reset)
	} else if warnings == 0 {
		fmt.Printf("  %s\u2713%s Everything looks good.\n", term.Green, term.Reset)
	} else {
		fmt.Printf("  %s%d warning(s) — see above.%s\n", term.Red, warnings, term.Reset)
	}
	fmt.Println()

	return nil
}

func runDoctorAISmoke(cfg config.Config) (string, error) {
	provider := cfg.Provider
	if provider == "auto" {
		provider = ai.AutoProviderName(cfg)
		if provider == "" {
			return "auto", fmt.Errorf("no available auto provider")
		}
	}

	rp, ok := cfg.ResolveProviderFull(provider)
	if !ok {
		return provider, fmt.Errorf("unknown provider")
	}

	label := doctorProviderLabel(cfg, provider)
	switch rp.Protocol {
	case config.ProtocolACP:
		return label, ai.CheckACPProvider(rp)
	case config.ProtocolPi:
		return label, ai.CheckPiProvider(rp)
	default:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := ai.FetchModels(ctx, provider, cfg)
		return label, err
	}
}

func doctorProviderLabel(cfg config.Config, provider string) string {
	cfg.Provider = provider
	return ai.ConfiguredProviderLabel(cfg)
}
