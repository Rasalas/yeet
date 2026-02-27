package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/term"
	"github.com/rasalas/yeet/internal/tui"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure yeet provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunAsCommit("config", args)
		}
		if err := tui.Run(); err != nil {
			return fmt.Errorf("config failed: %w", err)
		}
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config.toml in $EDITOR",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.Path()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
		editor := term.GetEditor()
		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("editor exited with error: %w", err)
		}
		fmt.Println("  \u2713 Config saved.")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configEditCmd)
	rootCmd.AddCommand(configCmd)
}
