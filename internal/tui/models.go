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
		providers: cfg.AllProviders(),
	}
}

func (t modelsTab) currentModel() string {
	p := t.providers[t.cursor]
	switch p {
	case "anthropic":
		return t.cfg.Anthropic.Model
	case "openai":
		return t.cfg.OpenAI.Model
	case "ollama":
		return t.cfg.Ollama.Model
	default:
		if custom, ok := t.cfg.Custom[p]; ok {
			return custom.Model
		}
		return ""
	}
}

func (t modelsTab) modelForProvider(p string) string {
	switch p {
	case "anthropic":
		return t.cfg.Anthropic.Model
	case "openai":
		return t.cfg.OpenAI.Model
	case "ollama":
		return t.cfg.Ollama.Model
	default:
		if custom, ok := t.cfg.Custom[p]; ok {
			return custom.Model
		}
		return ""
	}
}

func (t modelsTab) view() string {
	var b strings.Builder
	b.WriteString(styleLabel.Render("  Model per provider:"))
	b.WriteString("\n\n")

	for i, p := range t.providers {
		model := t.modelForProvider(p)

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
