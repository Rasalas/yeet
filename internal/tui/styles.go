package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("#7C3AED")
	colorSecondary = lipgloss.Color("#A78BFA")
	colorMuted     = lipgloss.Color("#6B7280")
	colorSuccess   = lipgloss.Color("#10B981")
	colorDanger    = lipgloss.Color("#EF4444")
	colorBg        = lipgloss.Color("#1F2937")
	colorText      = lipgloss.Color("#F9FAFB")

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			MarginBottom(1)

	styleActiveTab = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			Background(colorPrimary).
			Padding(0, 2)

	styleInactiveTab = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleNormal = lipgloss.NewStyle().
			Foreground(colorText)

	styleHelp = lipgloss.NewStyle().
			Foreground(colorMuted).
			MarginTop(1)

	styleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess)

	styleDanger = lipgloss.NewStyle().
			Foreground(colorDanger)

	styleLabel = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)
)
