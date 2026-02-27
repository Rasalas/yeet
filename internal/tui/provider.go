package tui

import (
	"fmt"
	"strings"

	"github.com/rasalas/yeet/internal/config"
)

type providerTab struct {
	providers []string
	cursor    int
	active    string
}

func newProviderTab(cfg config.Config) providerTab {
	providers := config.Providers()
	cursor := 0
	for i, p := range providers {
		if p == cfg.Provider {
			cursor = i
			break
		}
	}
	return providerTab{
		providers: providers,
		cursor:    cursor,
		active:    cfg.Provider,
	}
}

var providerLabels = map[string]string{
	"anthropic": "Anthropic Claude",
	"openai":    "OpenAI",
	"ollama":    "Ollama (local)",
}

func (t providerTab) view() string {
	var b strings.Builder
	b.WriteString(styleLabel.Render(fmt.Sprintf("  Active provider: %s", t.active)))
	b.WriteString("\n\n")
	for i, p := range t.providers {
		label := providerLabels[p]
		if label == "" {
			label = p
		}
		cursor := "    "
		if i == t.cursor {
			cursor = "  > "
		}
		style := styleNormal
		if p == t.active {
			style = styleSelected
		}
		b.WriteString(style.Render(cursor + label))
		b.WriteString("\n")
	}
	return b.String()
}
