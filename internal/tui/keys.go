package tui

import (
	"fmt"
	"strings"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

type keysTab struct {
	providers []string
	status    map[string]keyring.KeyInfo
	cursor    int
	editing   bool
	editBuf   string
	message   string
}

func newKeysTab(cfg config.Config) keysTab {
	providers := cfg.AllProviders()
	return keysTab{
		providers: providers,
		status:    keyring.Status(providers, cfg.CustomEnvs()),
	}
}

func (t keysTab) view() string {
	var b strings.Builder
	b.WriteString(styleLabel.Render("  API Key Status:"))
	b.WriteString("\n\n")

	for i, p := range t.providers {
		cursor := "    "
		if i == t.cursor {
			cursor = "  > "
		}

		info := t.status[p]
		icon := styleDanger.Render("✗")
		source := ""
		if info.Found {
			icon = styleSuccess.Render("✓")
			source = styleHelp.Render(fmt.Sprintf(" (%s)", info.Source))
		}

		label := providerLabels[p]
		if label == "" {
			label = p
		}

		if i == t.cursor && t.editing {
			b.WriteString(fmt.Sprintf("%s%s  %s: %s▌\n", cursor, icon, label, maskKey(t.editBuf)))
		} else {
			style := styleNormal
			if i == t.cursor {
				style = styleSelected
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s  %s", cursor, icon, label)))
			b.WriteString(source)
			b.WriteString("\n")
		}
	}

	if t.message != "" {
		b.WriteString("\n")
		b.WriteString("  " + t.message)
		b.WriteString("\n")
	}

	return b.String()
}

func maskKey(s string) string {
	return strings.Repeat("•", len(s))
}
