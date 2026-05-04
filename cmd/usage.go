package cmd

import (
	"strings"

	"github.com/rasalas/yeet/internal/ai"
)

func usageSummary(usage ai.Usage) string {
	var parts []string
	if usage.InputTokens > 0 || usage.OutputTokens > 0 {
		if cost, ok := usage.Cost(); ok {
			parts = append(parts, cost)
		}
		parts = append(parts, usage.FormatTokens())
	}
	if usage.Model != "" {
		parts = append(parts, usage.Model)
	}
	return strings.Join(parts, " · ")
}
