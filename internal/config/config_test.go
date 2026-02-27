package config

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != "auto" {
		t.Errorf("Provider = %q, want \"auto\"", cfg.Provider)
	}
	if cfg.Anthropic.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Anthropic.Model = %q", cfg.Anthropic.Model)
	}
	if cfg.OpenAI.Model != "gpt-4o-mini" {
		t.Errorf("OpenAI.Model = %q", cfg.OpenAI.Model)
	}
	if cfg.Ollama.Model != "llama3" {
		t.Errorf("Ollama.Model = %q", cfg.Ollama.Model)
	}
	if cfg.Ollama.URL != DefaultOllamaURL {
		t.Errorf("Ollama.URL = %q, want %q", cfg.Ollama.URL, DefaultOllamaURL)
	}
}

func TestDefaultModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", "claude-haiku-4-5-20251001"},
		{"openai", "gpt-4o-mini"},
		{"ollama", "llama3"},
		{"google", "gemini-3-flash-preview"},
		{"groq", "llama-3.3-70b-versatile"},
		{"nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := DefaultModel(tt.provider)
			if got != tt.want {
				t.Errorf("DefaultModel(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestResolveProvider(t *testing.T) {
	t.Run("well-known provider", func(t *testing.T) {
		cfg := DefaultConfig()
		pc, ok := cfg.ResolveProvider("google")
		if !ok {
			t.Fatal("ResolveProvider(google) returned false")
		}
		if pc.Model != "gemini-3-flash-preview" {
			t.Errorf("Model = %q", pc.Model)
		}
		if pc.Env != "GOOGLE_API_KEY" {
			t.Errorf("Env = %q", pc.Env)
		}
	})

	t.Run("custom overrides well-known", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"google": {Model: "gemini-custom", URL: "https://custom.example.com"},
		}
		pc, ok := cfg.ResolveProvider("google")
		if !ok {
			t.Fatal("ResolveProvider(google) returned false")
		}
		if pc.Model != "gemini-custom" {
			t.Errorf("Model = %q, want \"gemini-custom\"", pc.Model)
		}
		if pc.URL != "https://custom.example.com" {
			t.Errorf("URL = %q", pc.URL)
		}
		// Env should fall back to well-known since custom doesn't set it
		if pc.Env != "GOOGLE_API_KEY" {
			t.Errorf("Env = %q, want \"GOOGLE_API_KEY\"", pc.Env)
		}
	})

	t.Run("purely custom provider", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"together": {Model: "llama-70b", URL: "https://api.together.xyz/v1", Env: "TOGETHER_API_KEY"},
		}
		pc, ok := cfg.ResolveProvider("together")
		if !ok {
			t.Fatal("ResolveProvider(together) returned false")
		}
		if pc.Model != "llama-70b" {
			t.Errorf("Model = %q", pc.Model)
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		cfg := DefaultConfig()
		_, ok := cfg.ResolveProvider("nonexistent")
		if ok {
			t.Error("ResolveProvider(nonexistent) should return false")
		}
	})
}

func TestProviders(t *testing.T) {
	p := Providers()
	want := []string{"anthropic", "openai", "ollama"}
	if len(p) != len(want) {
		t.Fatalf("Providers() = %v, want %v", p, want)
	}
	for i, name := range want {
		if p[i] != name {
			t.Errorf("Providers()[%d] = %q, want %q", i, p[i], name)
		}
	}
}

func TestPricingOverrideDecode(t *testing.T) {
	input := `
provider = "openrouter"

[pricing."meta-llama/Llama-3-70b-chat-hf"]
input = 0.90
output = 0.90

[pricing."custom/cheap-model"]
input = 0.05
output = 0.10
`
	var cfg Config
	if _, err := toml.NewDecoder(strings.NewReader(input)).Decode(&cfg); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(cfg.Pricing) != 2 {
		t.Fatalf("Pricing has %d entries, want 2", len(cfg.Pricing))
	}
	llama := cfg.Pricing["meta-llama/Llama-3-70b-chat-hf"]
	if llama.Input != 0.90 || llama.Output != 0.90 {
		t.Errorf("llama pricing = {%v, %v}, want {0.90, 0.90}", llama.Input, llama.Output)
	}
	cheap := cfg.Pricing["custom/cheap-model"]
	if cheap.Input != 0.05 || cheap.Output != 0.10 {
		t.Errorf("cheap pricing = {%v, %v}, want {0.05, 0.10}", cheap.Input, cheap.Output)
	}
}

func TestCustomEnvs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Custom = map[string]ProviderConfig{
		"myapi": {Env: "MY_API_KEY"},
	}
	envs := cfg.CustomEnvs()

	// Well-known providers should be present
	if envs["google"] != "GOOGLE_API_KEY" {
		t.Errorf("google env = %q", envs["google"])
	}
	// Custom provider should be present
	if envs["myapi"] != "MY_API_KEY" {
		t.Errorf("myapi env = %q", envs["myapi"])
	}
}
