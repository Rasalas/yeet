package tui

import (
	"fmt"
	"strings"

	"github.com/rasalas/yeet/internal/config"
)

type modelsTab struct {
	cfg       config.Config
	providers []string
	cursor    int
	editing   bool
	editBuf   string
}

func newModelsTab(cfg config.Config) modelsTab {
	return modelsTab{
		cfg:       cfg,
		providers: config.Providers(),
	}
}

func (t modelsTab) currentModel() string {
	switch t.providers[t.cursor] {
	case "anthropic":
		return t.cfg.Anthropic.Model
	case "openai":
		return t.cfg.OpenAI.Model
	case "ollama":
		return t.cfg.Ollama.Model
	}
	return ""
}

func (t modelsTab) view() string {
	var b strings.Builder
	b.WriteString(styleLabel.Render("  Model per provider:"))
	b.WriteString("\n\n")

	for i, p := range t.providers {
		var model string
		switch p {
		case "anthropic":
			model = t.cfg.Anthropic.Model
		case "openai":
			model = t.cfg.OpenAI.Model
		case "ollama":
			model = t.cfg.Ollama.Model
		}

		cursor := "    "
		if i == t.cursor {
			cursor = "  > "
		}

		label := providerLabels[p]
		if label == "" {
			label = p
		}

		if i == t.cursor && t.editing {
			b.WriteString(fmt.Sprintf("%s%s: %s▌\n", cursor, label, t.editBuf))
		} else {
			style := styleNormal
			if i == t.cursor {
				style = styleSelected
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s: %s", cursor, label, model)))
			b.WriteString("\n")
		}
	}
	return b.String()
}
