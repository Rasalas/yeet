package config

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/xdg"
)

// DefaultOllamaURL is the default Ollama API endpoint.
const DefaultOllamaURL = "http://localhost:11434"

type ProviderConfig struct {
	Model string `toml:"model,omitempty"`
	URL   string `toml:"url,omitempty"`
	Env   string `toml:"env,omitempty"`
}

type PricingOverride struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
}

type Config struct {
	Provider  string                       `toml:"provider"`
	Anthropic ProviderConfig               `toml:"anthropic"`
	OpenAI    ProviderConfig               `toml:"openai"`
	Ollama    ProviderConfig               `toml:"ollama"`
	Custom    map[string]ProviderConfig    `toml:"custom"`
	Pricing   map[string]PricingOverride   `toml:"pricing"`
}

// WellKnown provides defaults for providers yeet recognizes but doesn't have
// builtin support for. They all use the OpenAI Chat Completions format.
var WellKnown = map[string]ProviderConfig{
	"google":     {Model: "gemini-3-flash-preview", URL: "https://generativelanguage.googleapis.com/v1beta/openai", Env: "GOOGLE_API_KEY"},
	"groq":       {Model: "llama-3.3-70b-versatile", URL: "https://api.groq.com/openai/v1", Env: "GROQ_API_KEY"},
	"openrouter": {Model: "openrouter/auto", URL: "https://openrouter.ai/api/v1", Env: "OPENROUTER_API_KEY"},
	"mistral":    {Model: "mistral-small-latest", URL: "https://api.mistral.ai/v1", Env: "MISTRAL_API_KEY"},
}

// KnownModels lists available models per provider for the TUI picker.
var KnownModels = map[string][]string{
	"anthropic":  {"claude-haiku-4-5-20251001", "claude-sonnet-4-6", "claude-opus-4-6"},
	"openai":     {"gpt-4o-mini", "gpt-4.1-nano", "gpt-4.1-mini", "gpt-4.1", "gpt-4o", "o4-mini"},
	"ollama":     {"llama3", "llama3.1", "gemma2", "mistral", "codellama", "qwen2.5-coder"},
	"google":     {"gemini-3-flash-preview", "gemini-2.5-flash"},
	"groq":       {"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "openai/gpt-oss-20b"},
	"openrouter": {"openrouter/auto", "google/gemini-3-flash-preview", "openai/gpt-4o-mini"},
	"mistral":    {"mistral-small-latest", "mistral-large-latest", "codestral-latest"},
}

func DefaultConfig() Config {
	return Config{
		Provider:  "auto",
		Anthropic: ProviderConfig{Model: "claude-haiku-4-5-20251001"},
		OpenAI:    ProviderConfig{Model: "gpt-4o-mini"},
		Ollama:    ProviderConfig{Model: "llama3", URL: DefaultOllamaURL},
	}
}

func configPath() (string, error) {
	dir, err := xdg.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Path returns the config file path, creating the file with defaults if it doesn't exist.
func Path() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := Save(DefaultConfig()); err != nil {
			return "", err
		}
	}
	return path, nil
}

func Load() (Config, error) {
	cfg := DefaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	_, err = toml.DecodeFile(path, &cfg)
	return cfg, err
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Don't persist models that match defaults — they'll auto-update with new versions.
	d := DefaultConfig()
	out := cfg
	if out.Anthropic.Model == d.Anthropic.Model {
		out.Anthropic.Model = ""
	}
	if out.OpenAI.Model == d.OpenAI.Model {
		out.OpenAI.Model = ""
	}
	if out.Ollama.Model == d.Ollama.Model {
		out.Ollama.Model = ""
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(out)
}

// Providers returns the builtin provider names.
func Providers() []string {
	return []string{"anthropic", "openai", "ollama"}
}

// AllProviders returns builtin + custom + discovered (OpenCode) provider names.
func (c Config) AllProviders() []string {
	builtin := Providers()
	seen := make(map[string]bool, len(builtin))
	for _, p := range builtin {
		seen[p] = true
	}

	var extra []string
	for name := range c.Custom {
		if !seen[name] {
			extra = append(extra, name)
			seen[name] = true
		}
	}
	for _, name := range keyring.OpenCodeProviders() {
		if !seen[name] {
			extra = append(extra, name)
			seen[name] = true
		}
	}

	sort.Strings(extra)
	return append(builtin, extra...)
}

// DefaultModel returns the default model for a builtin or well-known provider.
func DefaultModel(provider string) string {
	d := DefaultConfig()
	switch provider {
	case "anthropic":
		return d.Anthropic.Model
	case "openai":
		return d.OpenAI.Model
	case "ollama":
		return d.Ollama.Model
	}
	if wk, ok := WellKnown[provider]; ok {
		return wk.Model
	}
	return ""
}

// ResolveProvider returns the effective config for a non-builtin provider.
// Priority: user's [custom.X] config > WellKnown defaults.
// Fields are merged: user-set fields override, empty fields fall back to WellKnown.
func (c Config) ResolveProvider(name string) (ProviderConfig, bool) {
	custom, hasCustom := c.Custom[name]
	wk, hasWK := WellKnown[name]
	if !hasCustom && !hasWK {
		return ProviderConfig{}, false
	}
	// Start with well-known defaults, override with user config
	pc := wk
	if hasCustom {
		if custom.Model != "" {
			pc.Model = custom.Model
		}
		if custom.URL != "" {
			pc.URL = custom.URL
		}
		if custom.Env != "" {
			pc.Env = custom.Env
		}
	}
	return pc, true
}

// CustomEnvs returns a map of provider name to custom env var name.
// Includes well-known providers' env vars as fallback.
func (c Config) CustomEnvs() map[string]string {
	envs := make(map[string]string)
	for name, wk := range WellKnown {
		if wk.Env != "" {
			envs[name] = wk.Env
		}
	}
	for name, pc := range c.Custom {
		if pc.Env != "" {
			envs[name] = pc.Env
		}
	}
	return envs
}
