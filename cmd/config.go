package cmd

import (
	"fmt"

	"github.com/rasalas/yeet/internal/tui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure yeet (provider, model, keys)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunAsCommit("config", args)
		}
		if err := tui.Run(); err != nil {
			return fmt.Errorf("config TUI failed: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
