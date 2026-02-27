package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

type model struct {
	cfg         config.Config
	activeTab   int
	providerTab providerTab
	modelsTab   modelsTab
	keysTab     keysTab
	width       int
	height      int
	quitting    bool
}

func initialModel() model {
	cfg, _ := config.Load()
	return model{
		cfg:         cfg,
		providerTab: newProviderTab(cfg),
		modelsTab:   newModelsTab(cfg),
		keysTab:     newKeysTab(cfg),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "q", "ctrl+c":
			if m.activeTab == 1 && m.modelsTab.editing {
				break // don't quit while editing
			}
			if m.activeTab == 2 && m.keysTab.editing {
				break
			}
			m.quitting = true
			return m, tea.Quit
		case "tab", "right":
			if m.activeTab == 1 && m.modelsTab.editing {
				break
			}
			if m.activeTab == 2 && m.keysTab.editing {
				break
			}
			m.activeTab = (m.activeTab + 1) % len(tabNames)
			return m, nil
		case "shift+tab", "left":
			if m.activeTab == 1 && m.modelsTab.editing {
				break
			}
			if m.activeTab == 2 && m.keysTab.editing {
				break
			}
			m.activeTab = (m.activeTab - 1 + len(tabNames)) % len(tabNames)
			return m, nil
		}

		// Tab-specific keys
		switch m.activeTab {
		case 0:
			return m.updateProvider(msg)
		case 1:
			return m.updateModels(msg)
		case 2:
			return m.updateKeys(msg)
		}
	}

	return m, nil
}

func (m model) updateProvider(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerTab.cursor > 0 {
			m.providerTab.cursor--
		}
	case "down", "j":
		if m.providerTab.cursor < len(m.providerTab.providers)-1 {
			m.providerTab.cursor++
		}
	case "enter":
		selected := m.providerTab.providers[m.providerTab.cursor]
		m.providerTab.active = selected
		m.cfg.Provider = selected
		config.Save(m.cfg)
	}
	return m, nil
}

func (m model) updateModels(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.modelsTab.editing {
		switch msg.String() {
		case "enter":
			m.modelsTab.editing = false
			newModel := m.modelsTab.editBuf
			provider := m.modelsTab.providers[m.modelsTab.cursor]
			switch provider {
			case "anthropic":
				m.cfg.Anthropic.Model = newModel
			case "openai":
				m.cfg.OpenAI.Model = newModel
			case "ollama":
				m.cfg.Ollama.Model = newModel
			default:
				if m.cfg.Custom != nil {
					if custom, ok := m.cfg.Custom[provider]; ok {
						custom.Model = newModel
						m.cfg.Custom[provider] = custom
					}
				}
			}
			m.modelsTab.cfg = m.cfg
			config.Save(m.cfg)
		case "esc":
			m.modelsTab.editing = false
		case "backspace":
			if len(m.modelsTab.editBuf) > 0 {
				m.modelsTab.editBuf = m.modelsTab.editBuf[:len(m.modelsTab.editBuf)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.modelsTab.editBuf += msg.String()
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.modelsTab.cursor > 0 {
			m.modelsTab.cursor--
		}
	case "down", "j":
		if m.modelsTab.cursor < len(m.modelsTab.providers)-1 {
			m.modelsTab.cursor++
		}
	case "enter":
		m.modelsTab.editing = true
		m.modelsTab.editBuf = m.modelsTab.currentModel()
	}
	return m, nil
}

func (m model) updateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.keysTab.editing {
		switch msg.String() {
		case "enter":
			m.keysTab.editing = false
			provider := m.keysTab.providers[m.keysTab.cursor]
			key := m.keysTab.editBuf
			if key != "" {
				if err := keyring.Set(provider, key); err != nil {
					m.keysTab.message = styleDanger.Render(fmt.Sprintf("Failed to save: %v", err))
				} else {
					m.keysTab.message = styleSuccess.Render(fmt.Sprintf("✓ Key saved for %s", provider))
					m.keysTab.status[provider] = keyring.KeyInfo{Found: true, Source: keyring.SourceKeyring}
				}
			}
			m.keysTab.editBuf = ""
		case "esc":
			m.keysTab.editing = false
			m.keysTab.editBuf = ""
		case "backspace":
			if len(m.keysTab.editBuf) > 0 {
				m.keysTab.editBuf = m.keysTab.editBuf[:len(m.keysTab.editBuf)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.keysTab.editBuf += msg.String()
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		if m.keysTab.cursor > 0 {
			m.keysTab.cursor--
		}
	case "down", "j":
		if m.keysTab.cursor < len(m.keysTab.providers)-1 {
			m.keysTab.cursor++
		}
	case "enter":
		m.keysTab.editing = true
		m.keysTab.editBuf = ""
		m.keysTab.message = ""
	case "d", "delete":
		provider := m.keysTab.providers[m.keysTab.cursor]
		if err := keyring.Delete(provider); err != nil {
			m.keysTab.message = styleDanger.Render(fmt.Sprintf("Failed to delete: %v", err))
		} else {
			m.keysTab.message = styleSuccess.Render(fmt.Sprintf("✓ Key deleted for %s", provider))
			m.keysTab.status[provider] = keyring.KeyInfo{Found: false, Source: keyring.SourceNone}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(styleTitle.Render("  yeet config"))
	b.WriteString("\n")

	// Separator
	sep := strings.Repeat("─", max(m.width, 40))
	b.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(sep))
	b.WriteString("\n")

	// Tabs
	b.WriteString("  " + renderTabs(m.activeTab))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorMuted).Render(sep))
	b.WriteString("\n\n")

	// Content
	switch m.activeTab {
	case 0:
		b.WriteString(m.providerTab.view())
	case 1:
		b.WriteString(m.modelsTab.view())
	case 2:
		b.WriteString(m.keysTab.view())
	}

	// Help
	b.WriteString("\n")
	help := "  ←/→ tabs  ↑/↓ navigate  Enter select  q quit"
	switch m.activeTab {
	case 1:
		if m.modelsTab.editing {
			help = "  Type model name  Enter confirm  Esc cancel"
		} else {
			help = "  ←/→ tabs  ↑/↓ navigate  Enter edit  q quit"
		}
	case 2:
		if m.keysTab.editing {
			help = "  Type API key  Enter save  Esc cancel"
		} else {
			help = "  ←/→ tabs  ↑/↓ navigate  Enter set key  d delete  q quit"
		}
	}
	b.WriteString(styleHelp.Render(help))

	return b.String()
}

func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
