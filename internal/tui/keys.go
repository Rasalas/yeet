package tui

import (
	"fmt"
	"strings"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

type keysTab struct {
	providers []string
	status    map[string]bool
	cursor    int
	editing   bool
	editBuf   string
	message   string
}

func newKeysTab() keysTab {
	return keysTab{
		providers: config.Providers(),
		status:    keyring.Status(),
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

		icon := styleDanger.Render("✗")
		if t.status[p] {
			icon = styleSuccess.Render("✓")
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
