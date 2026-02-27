package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rasalas/yeet/internal/ai"
	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

var (
	colorPrimary   = lipgloss.Color("#7C3AED")
	colorSecondary = lipgloss.Color("#A78BFA")
	colorMuted     = lipgloss.Color("#6B7280")
	colorSuccess   = lipgloss.Color("#10B981")
	colorDanger    = lipgloss.Color("#EF4444")
	colorWarning   = lipgloss.Color("#F59E0B")
	colorText      = lipgloss.Color("#F9FAFB")

	styleTitle    = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	styleSelected = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleNormal   = lipgloss.NewStyle().Foreground(colorText)
	styleHelp     = lipgloss.NewStyle().Foreground(colorMuted)
	styleSuccess  = lipgloss.NewStyle().Foreground(colorSuccess)
	styleDanger   = lipgloss.NewStyle().Foreground(colorDanger)
	styleWarning  = lipgloss.NewStyle().Foreground(colorWarning)
	styleLabel    = lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)
)

type entry struct {
	name  string
	label string
	model string
	key   keyring.KeyInfo
}

type model struct {
	cfg      config.Config
	entries  []entry
	cursor   int
	message  string
	width    int
	height   int
	quitting bool

	// Model picker state
	picking      bool
	pickModels   []string // all models (from API or fallback)
	pickFiltered []string // filtered by pickFilter
	pickCursor   int      // cursor in pickFiltered
	pickFilter   string   // search text
	pickProvider string   // which provider
	pickLoading  bool     // show "Fetching..." spinner
}

// modelsLoadedMsg is sent when the async model fetch completes.
type modelsLoadedMsg struct {
	models []string
	err    error
}

var labels = map[string]string{
	"auto":       "Auto (cheapest available)",
	"anthropic":  "Anthropic",
	"openai":     "OpenAI",
	"ollama":     "Ollama (local)",
	"google":     "Google Gemini",
	"groq":       "Groq",
	"openrouter": "OpenRouter",
	"mistral":    "Mistral",
}

func initialModel() model {
	cfg, _ := config.Load()
	providers := append([]string{"auto"}, cfg.AllProviders()...)
	keyStatus := keyring.Status(cfg.AllProviders(), cfg.CustomEnvs())

	var entries []entry
	for _, p := range providers {
		label := labels[p]
		if label == "" {
			label = p
		}
		e := entry{name: p, label: label}
		if p == "auto" {
			e.model = ai.AutoModelName(cfg)
		} else {
			e.model = providerModel(cfg, p)
			e.key = keyStatus[p]
		}
		entries = append(entries, e)
	}

	cursor := 0
	for i, e := range entries {
		if e.name == cfg.Provider {
			cursor = i
			break
		}
	}

	return model{cfg: cfg, entries: entries, cursor: cursor}
}

func providerModel(cfg config.Config, p string) string {
	switch p {
	case "anthropic":
		return cfg.Anthropic.Model
	case "openai":
		return cfg.OpenAI.Model
	case "ollama":
		return cfg.Ollama.Model
	default:
		if pc, ok := cfg.ResolveProvider(p); ok {
			return pc.Model
		}
		return ""
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case modelsLoadedMsg:
		return m.handleModelsLoaded(msg)
	case tea.KeyMsg:
		if m.picking {
			return m.updatePicking(msg)
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.message = ""
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.message = ""
			}
		case "enter":
			selected := m.entries[m.cursor]
			m.cfg.Provider = selected.name
			config.Save(m.cfg)
			m.message = styleSuccess.Render(fmt.Sprintf("  ✓ Provider set to %s", selected.label))
		case "m":
			e := m.entries[m.cursor]
			if e.name == "auto" {
				break
			}
			m.picking = true
			m.pickProvider = e.name
			m.pickLoading = true
			m.pickFilter = ""
			m.pickModels = nil
			m.pickFiltered = nil
			m.pickCursor = 0
			m.message = ""
			return m, m.fetchModelsCmd()
		case "r":
			e := m.entries[m.cursor]
			def := config.DefaultModel(e.name)
			if def == "" || e.name == "auto" {
				break
			}
			m.saveModel(e.name, def)
			m.entries[m.cursor].model = def
			m.message = styleSuccess.Render(fmt.Sprintf("  ✓ %s reset to %s", e.label, def))
		}
	}
	return m, nil
}

func (m *model) fetchModelsCmd() tea.Cmd {
	provider := m.pickProvider
	cfg := m.cfg
	return func() tea.Msg {
		models, err := ai.FetchModels(provider, cfg)
		return modelsLoadedMsg{models: models, err: err}
	}
}

func (m *model) handleModelsLoaded(msg modelsLoadedMsg) (tea.Model, tea.Cmd) {
	m.pickLoading = false
	if msg.err != nil || len(msg.models) == 0 {
		// Fallback to static KnownModels
		m.pickModels = config.KnownModels[m.pickProvider]
	} else {
		m.pickModels = msg.models
	}
	m.applyFilter()
	return m, nil
}

// fuzzyMatch checks if all characters in pattern appear in str in order (case-insensitive).
func fuzzyMatch(str, pattern string) bool {
	str = strings.ToLower(str)
	pattern = strings.ToLower(pattern)
	pi := 0
	for i := 0; i < len(str) && pi < len(pattern); i++ {
		if str[i] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

func (m *model) applyFilter() {
	if m.pickFilter == "" {
		m.pickFiltered = m.pickModels
	} else {
		filtered := make([]string, 0)
		for _, name := range m.pickModels {
			if fuzzyMatch(name, m.pickFilter) {
				filtered = append(filtered, name)
			}
		}
		m.pickFiltered = filtered
	}

	// Determine total items (filtered models + optional "Use X" entry)
	total := m.pickListLen()

	// Try to place cursor on current model
	currentModel := m.entries[m.cursor].model
	placed := false
	for i, name := range m.pickFiltered {
		if name == currentModel {
			m.pickCursor = i
			placed = true
			break
		}
	}
	if !placed {
		m.pickCursor = 0
	}

	// Clamp
	if m.pickCursor >= total {
		m.pickCursor = max(total-1, 0)
	}
}

// pickListLen returns the total number of selectable items in the picker.
func (m *model) pickListLen() int {
	n := len(m.pickFiltered)
	if m.pickFilter != "" {
		n++ // "Use X as model name" entry
	}
	return n
}

// pickIsUseCustom returns true if the cursor is on the "Use X" entry.
func (m *model) pickIsUseCustom() bool {
	return m.pickFilter != "" && m.pickCursor == len(m.pickFiltered)
}

func (m *model) updatePicking(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While loading, only allow escape
	if m.pickLoading {
		if msg.String() == "esc" || msg.String() == "ctrl+c" {
			m.picking = false
			m.pickLoading = false
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.picking = false
		m.pickModels = nil
		m.pickFiltered = nil
		m.pickFilter = ""
	case "up":
		if m.pickCursor > 0 {
			m.pickCursor--
		}
	case "down":
		if m.pickCursor < m.pickListLen()-1 {
			m.pickCursor++
		}
	case "enter":
		var chosen string
		if m.pickIsUseCustom() {
			chosen = m.pickFilter
		} else if len(m.pickFiltered) > 0 && m.pickCursor < len(m.pickFiltered) {
			chosen = m.pickFiltered[m.pickCursor]
		} else {
			return m, nil
		}
		m.picking = false
		e := m.entries[m.cursor]
		m.saveModel(e.name, chosen)
		m.entries[m.cursor].model = chosen
		m.message = styleSuccess.Render(fmt.Sprintf("  ✓ Model for %s set to %s", e.label, chosen))
		m.pickModels = nil
		m.pickFiltered = nil
		m.pickFilter = ""
	case "backspace":
		if len(m.pickFilter) > 0 {
			m.pickFilter = m.pickFilter[:len(m.pickFilter)-1]
			m.applyFilter()
		}
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 {
			m.pickFilter += key
			m.applyFilter()
		}
	}
	return m, nil
}

func (m *model) saveModel(provider, newModel string) {
	switch provider {
	case "anthropic":
		m.cfg.Anthropic.Model = newModel
	case "openai":
		m.cfg.OpenAI.Model = newModel
	case "ollama":
		m.cfg.Ollama.Model = newModel
	default:
		if m.cfg.Custom == nil {
			m.cfg.Custom = make(map[string]config.ProviderConfig)
		}
		pc := m.cfg.Custom[provider]
		pc.Model = newModel
		// Inherit URL/Env from well-known if not set
		if wk, ok := config.WellKnown[provider]; ok {
			if pc.URL == "" {
				pc.URL = wk.URL
			}
			if pc.Env == "" {
				pc.Env = wk.Env
			}
		}
		m.cfg.Custom[provider] = pc
	}
	config.Save(m.cfg)
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	// Model picker overlay
	if m.picking {
		return m.viewModelPicker()
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(styleTitle.Render("  Which provider should generate commit messages?"))
	b.WriteString("\n\n")

	active := m.cfg.Provider

	for i, e := range m.entries {
		cursor := "    "
		if i == m.cursor {
			cursor = "  > "
		}

		style := styleNormal
		if i == m.cursor {
			style = styleSelected
		}

		b.WriteString(style.Render(cursor + e.label))

		// Model
		if e.name == "auto" {
			if e.model != "" {
				b.WriteString(styleHelp.Render("  → " + e.model))
			}
		} else if e.model != "" {
			def := config.DefaultModel(e.name)
			if def != "" && e.model != def {
				b.WriteString(styleWarning.Render("  " + e.model))
				b.WriteString(styleHelp.Render(fmt.Sprintf("  (default: %s, r to reset)", def)))
			} else {
				b.WriteString(styleHelp.Render("  " + e.model))
			}
		}

		// Key status
		if e.name != "auto" && e.name != "ollama" {
			if e.key.Found {
				b.WriteString("  " + styleSuccess.Render("✓"))
			} else {
				b.WriteString("  " + styleDanger.Render("✗ no key"))
			}
		}

		// Active marker
		if e.name == active {
			b.WriteString(styleLabel.Render("  ← active"))
		}

		b.WriteString("\n")

		// Separator after "auto"
		if e.name == "auto" {
			sep := strings.Repeat("─", max(m.width-4, 40))
			b.WriteString(styleHelp.Render("  " + sep))
			b.WriteString("\n")
		}
	}

	if m.message != "" {
		b.WriteString("\n" + m.message + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("  ↑/↓ navigate · Enter set provider · m change model · q quit"))
	b.WriteString("\n")

	return b.String()
}

func (m model) viewModelPicker() string {
	var b strings.Builder

	e := m.entries[m.cursor]

	b.WriteString("\n")
	b.WriteString(styleTitle.Render(fmt.Sprintf("  Select model for %s", e.label)))
	b.WriteString("\n\n")

	// Loading state
	if m.pickLoading {
		b.WriteString(styleHelp.Render("  Fetching models..."))
		b.WriteString("\n\n")
		b.WriteString(styleHelp.Render("  Esc cancel"))
		b.WriteString("\n")
		return b.String()
	}

	// Filter field
	b.WriteString(styleHelp.Render("  Filter: "))
	b.WriteString(m.pickFilter)
	b.WriteString("▌")
	b.WriteString("\n\n")

	// Model list with viewport scrolling
	// Reserve lines for: title(2) + filter(2) + separator+useX(2) + help(2) + padding(2) = ~10
	maxVisible := m.height - 10
	if maxVisible < 5 {
		maxVisible = 5
	}

	currentModel := e.model
	defModel := config.DefaultModel(e.name)

	// Calculate scroll window centered on cursor (only for model entries, not "Use X")
	modelCount := len(m.pickFiltered)
	scrollStart := 0
	if modelCount > maxVisible {
		// Keep cursor roughly centered
		scrollStart = m.pickCursor - maxVisible/2
		if scrollStart < 0 {
			scrollStart = 0
		}
		if scrollStart > modelCount-maxVisible {
			scrollStart = modelCount - maxVisible
		}
	}
	scrollEnd := scrollStart + maxVisible
	if scrollEnd > modelCount {
		scrollEnd = modelCount
	}

	if scrollStart > 0 {
		b.WriteString(styleHelp.Render(fmt.Sprintf("    ↑ %d more", scrollStart)))
		b.WriteString("\n")
	}

	for i := scrollStart; i < scrollEnd; i++ {
		name := m.pickFiltered[i]
		cursor := "    "
		if i == m.pickCursor {
			cursor = "  > "
		}

		style := styleNormal
		if i == m.pickCursor {
			style = styleSelected
		}

		b.WriteString(style.Render(cursor + name))

		if name == currentModel {
			b.WriteString(styleLabel.Render("  ← current"))
		} else if defModel != "" && name == defModel {
			b.WriteString(styleHelp.Render("  (default)"))
		}

		b.WriteString("\n")
	}

	if scrollEnd < modelCount {
		b.WriteString(styleHelp.Render(fmt.Sprintf("    ↓ %d more", modelCount-scrollEnd)))
		b.WriteString("\n")
	}

	// "Use X as model name" entry (only when filter is non-empty)
	if m.pickFilter != "" {
		if len(m.pickFiltered) > 0 {
			sep := strings.Repeat("─", max(m.width-4, 40))
			b.WriteString(styleHelp.Render("    " + sep))
			b.WriteString("\n")
		}

		idx := len(m.pickFiltered)
		cursor := "    "
		if m.pickCursor == idx {
			cursor = "  > "
		}

		style := styleNormal
		if m.pickCursor == idx {
			style = styleSelected
		}

		b.WriteString(style.Render(fmt.Sprintf(`%sUse "%s" as model name`, cursor, m.pickFilter)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styleHelp.Render("  ↑/↓ navigate · Enter select · Esc back"))
	b.WriteString("\n")

	return b.String()
}

func Run() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
