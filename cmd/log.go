package cmd

import (
	"fmt"

	"github.com/rasalas/yeet/internal/git"
	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Show recent commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunAsCommit("log", args)
		}
		out, err := git.Log()
		if err != nil {
			return fmt.Errorf("git log failed: %w", err)
		}
		fmt.Println()
		fmt.Println(out)
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(logCmd)
}
