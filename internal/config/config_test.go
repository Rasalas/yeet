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
	if cfg.Ollama.URL != Registry["ollama"].DefaultURL {
		t.Errorf("Ollama.URL = %q, want %q", cfg.Ollama.URL, Registry["ollama"].DefaultURL)
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

func TestResolveProviderFull(t *testing.T) {
	t.Run("builtin with struct overrides", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Anthropic.Model = "claude-opus-4-6"
		rp, ok := cfg.ResolveProviderFull("anthropic")
		if !ok {
			t.Fatal("returned false")
		}
		if rp.Model != "claude-opus-4-6" {
			t.Errorf("Model = %q, want \"claude-opus-4-6\"", rp.Model)
		}
		if rp.Protocol != ProtocolAnthropic {
			t.Errorf("Protocol = %q", rp.Protocol)
		}
		if !rp.NeedsAuth {
			t.Error("NeedsAuth should be true")
		}
		if rp.URL != "https://api.anthropic.com/v1" {
			t.Errorf("URL = %q", rp.URL)
		}
	})

	t.Run("registry provider without custom", func(t *testing.T) {
		cfg := DefaultConfig()
		rp, ok := cfg.ResolveProviderFull("groq")
		if !ok {
			t.Fatal("returned false")
		}
		if rp.Model != "llama-3.3-70b-versatile" {
			t.Errorf("Model = %q", rp.Model)
		}
		if rp.Protocol != ProtocolOpenAI {
			t.Errorf("Protocol = %q", rp.Protocol)
		}
	})

	t.Run("custom overrides registry", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"groq": {Model: "custom-model"},
		}
		rp, ok := cfg.ResolveProviderFull("groq")
		if !ok {
			t.Fatal("returned false")
		}
		if rp.Model != "custom-model" {
			t.Errorf("Model = %q, want \"custom-model\"", rp.Model)
		}
		// URL should fall back to registry
		if rp.URL != "https://api.groq.com/openai/v1" {
			t.Errorf("URL = %q", rp.URL)
		}
	})

	t.Run("ollama is no-auth", func(t *testing.T) {
		cfg := DefaultConfig()
		rp, ok := cfg.ResolveProviderFull("ollama")
		if !ok {
			t.Fatal("returned false")
		}
		if rp.NeedsAuth {
			t.Error("NeedsAuth should be false for ollama")
		}
		if rp.Protocol != ProtocolOllama {
			t.Errorf("Protocol = %q", rp.Protocol)
		}
	})

	t.Run("purely custom defaults to openai protocol", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"together": {Model: "llama-70b", URL: "https://api.together.xyz/v1", Env: "TOGETHER_API_KEY"},
		}
		rp, ok := cfg.ResolveProviderFull("together")
		if !ok {
			t.Fatal("returned false")
		}
		if rp.Protocol != ProtocolOpenAI {
			t.Errorf("Protocol = %q, want openai", rp.Protocol)
		}
		if !rp.NeedsAuth {
			t.Error("NeedsAuth should be true for custom provider")
		}
	})

	t.Run("unknown provider returns false", func(t *testing.T) {
		cfg := DefaultConfig()
		_, ok := cfg.ResolveProviderFull("nonexistent")
		if ok {
			t.Error("should return false for unknown provider")
		}
	})
}

func TestSetModel(t *testing.T) {
	t.Run("builtin anthropic", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SetModel("anthropic", "claude-opus-4-6")
		if cfg.Anthropic.Model != "claude-opus-4-6" {
			t.Errorf("Anthropic.Model = %q", cfg.Anthropic.Model)
		}
	})

	t.Run("builtin openai", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SetModel("openai", "gpt-4o")
		if cfg.OpenAI.Model != "gpt-4o" {
			t.Errorf("OpenAI.Model = %q", cfg.OpenAI.Model)
		}
	})

	t.Run("builtin ollama", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SetModel("ollama", "mistral")
		if cfg.Ollama.Model != "mistral" {
			t.Errorf("Ollama.Model = %q", cfg.Ollama.Model)
		}
	})

	t.Run("registry provider goes to custom", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SetModel("groq", "llama-custom")
		pc := cfg.Custom["groq"]
		if pc.Model != "llama-custom" {
			t.Errorf("Custom[groq].Model = %q", pc.Model)
		}
		if pc.URL != "https://api.groq.com/openai/v1" {
			t.Errorf("Custom[groq].URL = %q", pc.URL)
		}
		if pc.Env != "GROQ_API_KEY" {
			t.Errorf("Custom[groq].Env = %q", pc.Env)
		}
	})

	t.Run("purely custom provider", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SetModel("together", "llama-70b")
		pc := cfg.Custom["together"]
		if pc.Model != "llama-70b" {
			t.Errorf("Custom[together].Model = %q", pc.Model)
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Provider = "anthropic"
		problems := cfg.Validate()
		if len(problems) != 0 {
			t.Errorf("unexpected problems: %v", problems)
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Provider = "nonexistent"
		problems := cfg.Validate()
		found := false
		for _, p := range problems {
			if strings.Contains(p, "unknown provider") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected unknown provider warning, got: %v", problems)
		}
	})

	t.Run("custom missing url", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"myapi": {Model: "test", Env: "MY_KEY"},
		}
		problems := cfg.Validate()
		found := false
		for _, p := range problems {
			if strings.Contains(p, "missing url") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected missing url warning, got: %v", problems)
		}
	})

	t.Run("custom missing env", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"myapi": {Model: "test", URL: "https://example.com"},
		}
		problems := cfg.Validate()
		found := false
		for _, p := range problems {
			if strings.Contains(p, "no env var") {
				found = true
			}
		}
		if !found {
			t.Errorf("expected missing env warning, got: %v", problems)
		}
	})

	t.Run("registry override in custom is fine", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Custom = map[string]ProviderConfig{
			"groq": {Model: "custom-model"},
		}
		problems := cfg.Validate()
		if len(problems) != 0 {
			t.Errorf("unexpected problems for registry override: %v", problems)
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

	// Registry providers should be present
	if envs["google"] != "GOOGLE_API_KEY" {
		t.Errorf("google env = %q", envs["google"])
	}
	if envs["anthropic"] != "ANTHROPIC_API_KEY" {
		t.Errorf("anthropic env = %q", envs["anthropic"])
	}
	// Custom provider should be present
	if envs["myapi"] != "MY_API_KEY" {
		t.Errorf("myapi env = %q", envs["myapi"])
	}
}
