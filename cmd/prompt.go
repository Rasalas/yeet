package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/spf13/cobra"
)

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Edit the AI system prompt",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return RunAsCommit("prompt", args)
		}

		// Ensure the file exists with default content
		ai.LoadPrompt()

		path, err := ai.PromptPath()
		if err != nil {
			return err
		}

		editor := getEditor()
		c := exec.Command(editor, path)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("editor exited with error: %w", err)
		}

		fmt.Printf("  %s✓%s Prompt saved.\n", green, reset)
		return nil
	},
}

var promptResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the prompt to the default",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ai.WritePrompt(ai.DefaultPrompt); err != nil {
			return fmt.Errorf("failed to reset prompt: %w", err)
		}
		fmt.Printf("  %s✓%s Prompt reset to default.\n", green, reset)
		return nil
	},
}

var promptShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current prompt",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println()
		fmt.Println(ai.LoadPrompt())
		fmt.Println()
	},
}

func init() {
	promptCmd.AddCommand(promptResetCmd)
	promptCmd.AddCommand(promptShowCmd)
	rootCmd.AddCommand(promptCmd)
}
