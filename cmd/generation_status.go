package cmd

import (
	"fmt"

	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/term"
)

func generationLabel(action, provider string) string {
	if provider == "" {
		return action + "..."
	}
	return fmt.Sprintf("%s with %s...", action, provider)
}

func configureAttemptStatus(provider any, spinner *term.Spinner, started *bool, action, initialLabel string) {
	reporter, ok := provider.(ai.AttemptReporter)
	if !ok {
		return
	}

	currentLabel := initialLabel
	reporter.SetAttemptCallback(func(attempt ai.ProviderAttempt) {
		if *started {
			return
		}

		nextLabel := attempt.Label()
		if attempt.Previous != nil {
			spinner.Stop()
			fmt.Printf("  %s%s failed, falling back to %s%s\n", term.Dim, attempt.Previous.Name, nextLabel, term.Reset)
			spinner.Start(generationLabel(action, nextLabel))
			currentLabel = nextLabel
			return
		}

		if nextLabel != "" && nextLabel != currentLabel {
			spinner.Stop()
			spinner.Start(generationLabel(action, nextLabel))
			currentLabel = nextLabel
		}
	})
}
