package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rasalas/yeet/internal/config"
)

func TestProviderViewFitsTerminalHeightAndKeepsCursorVisible(t *testing.T) {
	m := providerViewTestModel(9, 4, 12)
	view := m.View()

	if got := lipgloss.Height(view); got > m.height {
		t.Fatalf("view height = %d, terminal height = %d\n%s", got, m.height, view)
	}
	if !strings.Contains(view, "provider-04") {
		t.Fatal("cursor provider is not visible")
	}
	if strings.Contains(view, "provider-00") || strings.Contains(view, "provider-08") {
		t.Fatal("small viewport should hide entries far from the cursor")
	}
	if !strings.Contains(view, "more providers") {
		t.Fatal("small viewport should show overflow indicators")
	}
	for i := 0; i < 9; i++ {
		label := fmt.Sprintf("provider-%02d", i)
		if count := strings.Count(view, label); count > 1 {
			t.Fatalf("%s rendered %d times", label, count)
		}
	}

	m.message = "  saved"
	if got := lipgloss.Height(m.View()); got > m.height {
		t.Fatalf("view with status height = %d, terminal height = %d", got, m.height)
	}
}

func TestProviderViewExpandsAfterTerminalResizeWithoutDuplicates(t *testing.T) {
	m := providerViewTestModel(9, 4, 40)
	view := m.View()

	if strings.Contains(view, "more providers") {
		t.Fatal("large viewport should not show overflow indicators")
	}
	for i := 0; i < 9; i++ {
		label := fmt.Sprintf("provider-%02d", i)
		if count := strings.Count(view, label); count != 1 {
			t.Fatalf("%s rendered %d times, want 1", label, count)
		}
	}
}

func TestProviderNavigationClearsScreenOnlyWhenViewportMoves(t *testing.T) {
	small := providerViewTestModel(9, 4, 12)
	updated, cmd := small.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd == nil {
		t.Fatal("viewport shift should clear the screen")
	}
	if got := updated.(model).cursor; got != 5 {
		t.Fatalf("cursor = %d, want 5", got)
	}

	large := providerViewTestModel(9, 4, 40)
	_, cmd = large.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("navigation inside an unchanged viewport should use incremental rendering")
	}
}

func TestProviderResizeClearsScreen(t *testing.T) {
	initial := providerViewTestModel(9, 4, 0)
	initial.width = 0
	_, cmd := initial.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	if cmd != nil {
		t.Fatal("initial size message should rely on the alt-screen repaint")
	}

	m := providerViewTestModel(9, 4, 20)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 15})
	if cmd == nil {
		t.Fatal("terminal resize should clear the screen")
	}
	got := updated.(model)
	if got.width != 100 || got.height != 15 {
		t.Fatalf("size = %dx%d, want 100x15", got.width, got.height)
	}
}

func providerViewTestModel(count, cursor, height int) model {
	entries := make([]entry, count)
	for i := range entries {
		name := fmt.Sprintf("provider-%02d", i)
		entries[i] = entry{name: name, label: name}
	}
	return model{
		cfg:     config.Config{Provider: entries[cursor].name},
		entries: entries,
		cursor:  cursor,
		width:   80,
		height:  height,
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		str, pattern string
		want         bool
	}{
		{"claude-haiku-4-5", "haiku", true},
		{"claude-haiku-4-5", "HAIKU", true},
		{"claude-haiku-4-5", "ch45", true},
		{"gpt-4o-mini", "4om", true},
		{"gpt-4o-mini", "xyz", false},
		{"claude-haiku-4-5", "", true},
		{"", "abc", false},
		{"", "", true},
		{"llama3", "llama3", true},
		{"llama3", "llama4", false},
	}

	for _, tt := range tests {
		got := fuzzyMatch(tt.str, tt.pattern)
		if got != tt.want {
			t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.str, tt.pattern, got, tt.want)
		}
	}
}

func TestPickListLen(t *testing.T) {
	m := model{
		pickFiltered: []string{"a", "b", "c"},
		pickFilter:   "",
	}

	if got := m.pickListLen(); got != 3 {
		t.Errorf("pickListLen without filter = %d, want 3", got)
	}

	m.pickFilter = "a"
	if got := m.pickListLen(); got != 4 {
		t.Errorf("pickListLen with filter = %d, want 4 (3 models + 1 'Use X')", got)
	}
}

func TestPickIsUseCustom(t *testing.T) {
	m := model{
		pickFiltered: []string{"a", "b"},
		pickFilter:   "custom",
		pickCursor:   2, // index == len(pickFiltered) → "Use X" entry
	}

	if !m.pickIsUseCustom() {
		t.Error("expected pickIsUseCustom() = true when cursor is on custom entry")
	}

	m.pickCursor = 1
	if m.pickIsUseCustom() {
		t.Error("expected pickIsUseCustom() = false when cursor is on a model")
	}

	// No filter → no custom entry
	m.pickFilter = ""
	m.pickCursor = 2
	if m.pickIsUseCustom() {
		t.Error("expected pickIsUseCustom() = false when filter is empty")
	}
}

func TestPrependNativeModelChoice(t *testing.T) {
	got := prependNativeModelChoice([]string{"gpt-5.4-mini", nativeModelChoice})
	want := []string{nativeModelChoice, "gpt-5.4-mini"}
	if len(got) != len(want) {
		t.Fatalf("prependNativeModelChoice length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("prependNativeModelChoice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestProviderModel(t *testing.T) {
	cfg := config.Config{
		Anthropic: config.ProviderConfig{Model: "claude-haiku-4-5-20251001"},
		OpenAI:    config.ProviderConfig{Model: "gpt-4o-mini"},
		Ollama:    config.ProviderConfig{Model: "llama3"},
		Custom: map[string]config.ProviderConfig{
			"groq": {Model: "llama-3.3-70b-versatile", URL: "https://api.groq.com/openai/v1", Env: "GROQ_API_KEY"},
		},
	}

	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", "claude-haiku-4-5-20251001"},
		{"openai", "gpt-4o-mini"},
		{"ollama", "llama3"},
		{"groq", "llama-3.3-70b-versatile"},
	}

	for _, tt := range tests {
		got := providerModel(cfg, tt.provider)
		if got != tt.want {
			t.Errorf("providerModel(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}
}

func TestProviderModelUnknown(t *testing.T) {
	cfg := config.Config{}
	got := providerModel(cfg, "nonexistent")
	if got != "" {
		t.Errorf("providerModel(nonexistent) = %q, want empty", got)
	}
}

func TestProviderReasoningEffort(t *testing.T) {
	cfg := config.DefaultConfig()
	if got := providerReasoningEffort(cfg, "codex"); got != "low" {
		t.Errorf("providerReasoningEffort(codex) = %q, want low", got)
	}
	cfg.SetReasoningEffort("codex", "high")
	if got := providerReasoningEffort(cfg, "codex"); got != "high" {
		t.Errorf("providerReasoningEffort(codex) = %q, want high", got)
	}
}

func TestProviderUpstream(t *testing.T) {
	cfg := config.DefaultConfig()
	if got := providerUpstream(cfg, "pi"); got != "openai-codex" {
		t.Fatalf("providerUpstream(pi) = %q", got)
	}
	cfg.SetUpstream("pi", "anthropic")
	if got := providerUpstream(cfg, "pi"); got != "anthropic" {
		t.Fatalf("providerUpstream(pi) = %q after update", got)
	}
	if got := fallbackModels(cfg, "pi"); len(got) == 0 || got[0] != "claude-fable-5" {
		t.Fatalf("Pi Anthropic fallback models = %v", got)
	}
}

func TestPiUpstreamFallbackKeepsCurrentCustomProvider(t *testing.T) {
	m := model{
		entries: []entry{{name: "pi", upstream: "company-proxy"}},
		cursor:  0,
	}

	_, _ = m.handlePiUpstreamsLoaded(piUpstreamsLoadedMsg{err: fmt.Errorf("Pi unavailable")})

	if len(m.upstreamChoices) == 0 || m.upstreamChoices[0] != "company-proxy" {
		t.Fatalf("upstream choices = %v, want current custom provider first", m.upstreamChoices)
	}
	if m.upstreamCursor != 0 {
		t.Fatalf("upstream cursor = %d, want 0", m.upstreamCursor)
	}
}

func TestProviderSupportsReasoningEffort(t *testing.T) {
	cfg := config.DefaultConfig()
	if !providerSupportsReasoningEffort(cfg, "codex") {
		t.Fatal("expected codex to support reasoning effort")
	}
	if providerSupportsReasoningEffort(cfg, "claude") {
		t.Fatal("expected claude to not expose yeet-managed reasoning effort")
	}
}
