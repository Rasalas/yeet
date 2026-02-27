package tui

import (
	"testing"

	"github.com/rasalas/yeet/internal/config"
)

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
