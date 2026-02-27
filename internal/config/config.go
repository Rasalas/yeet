package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/xdg"
)

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
	Provider  string                     `toml:"provider"`
	Anthropic ProviderConfig             `toml:"anthropic"`
	OpenAI    ProviderConfig             `toml:"openai"`
	Ollama    ProviderConfig             `toml:"ollama"`
	Custom    map[string]ProviderConfig  `toml:"custom"`
	Pricing   map[string]PricingOverride `toml:"pricing"`
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
		Anthropic: ProviderConfig{Model: Registry["anthropic"].DefaultModel},
		OpenAI:    ProviderConfig{Model: Registry["openai"].DefaultModel},
		Ollama:    ProviderConfig{Model: Registry["ollama"].DefaultModel, URL: Registry["ollama"].DefaultURL},
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
	out := cfg
	for _, name := range Providers() {
		entry, ok := Registry[name]
		if !ok {
			continue
		}
		switch name {
		case "anthropic":
			if out.Anthropic.Model == entry.DefaultModel {
				out.Anthropic.Model = ""
			}
		case "openai":
			if out.OpenAI.Model == entry.DefaultModel {
				out.OpenAI.Model = ""
			}
		case "ollama":
			if out.Ollama.Model == entry.DefaultModel {
				out.Ollama.Model = ""
			}
		}
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
	for name := range Registry {
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

// DefaultModel returns the default model for a known provider.
func DefaultModel(provider string) string {
	if entry, ok := Registry[provider]; ok {
		return entry.DefaultModel
	}
	return ""
}

// ResolveProviderFull returns the fully-resolved provider configuration.
// Three-layer merge: Registry defaults → named struct fields (builtins) → Custom overrides.
// Purely custom providers (not in Registry) default to ProtocolOpenAI + NeedsAuth.
func (c Config) ResolveProviderFull(name string) (ResolvedProvider, bool) {
	entry, inRegistry := Registry[name]
	custom, hasCustom := c.Custom[name]

	// Start from registry defaults
	rp := ResolvedProvider{
		Name:      name,
		Model:     entry.DefaultModel,
		URL:       entry.DefaultURL,
		Env:       entry.DefaultEnv,
		Protocol:  entry.Protocol,
		NeedsAuth: entry.NeedsAuth,
	}

	// Layer 2: named struct fields for builtins
	switch name {
	case "anthropic":
		if c.Anthropic.Model != "" {
			rp.Model = c.Anthropic.Model
		}
		if c.Anthropic.URL != "" {
			rp.URL = c.Anthropic.URL
		}
		if c.Anthropic.Env != "" {
			rp.Env = c.Anthropic.Env
		}
	case "openai":
		if c.OpenAI.Model != "" {
			rp.Model = c.OpenAI.Model
		}
		if c.OpenAI.URL != "" {
			rp.URL = c.OpenAI.URL
		}
		if c.OpenAI.Env != "" {
			rp.Env = c.OpenAI.Env
		}
	case "ollama":
		if c.Ollama.Model != "" {
			rp.Model = c.Ollama.Model
		}
		if c.Ollama.URL != "" {
			rp.URL = c.Ollama.URL
		}
		if c.Ollama.Env != "" {
			rp.Env = c.Ollama.Env
		}
	}

	// Layer 3: Custom overrides (covers well-known overrides + purely custom)
	if hasCustom {
		if custom.Model != "" {
			rp.Model = custom.Model
		}
		if custom.URL != "" {
			rp.URL = custom.URL
		}
		if custom.Env != "" {
			rp.Env = custom.Env
		}
	}

	// Purely custom provider not in registry
	if !inRegistry && !hasCustom {
		return ResolvedProvider{}, false
	}
	if !inRegistry {
		rp.Protocol = ProtocolOpenAI
		rp.NeedsAuth = true
	}

	return rp, true
}

// ResolveProvider returns the effective config for a non-builtin provider.
// Kept for backward compatibility; prefer ResolveProviderFull.
func (c Config) ResolveProvider(name string) (ProviderConfig, bool) {
	rp, ok := c.ResolveProviderFull(name)
	if !ok {
		return ProviderConfig{}, false
	}
	return ProviderConfig{Model: rp.Model, URL: rp.URL, Env: rp.Env}, true
}

// SetModel writes a model to the appropriate config location.
func (c *Config) SetModel(provider, model string) {
	switch provider {
	case "anthropic":
		c.Anthropic.Model = model
	case "openai":
		c.OpenAI.Model = model
	case "ollama":
		c.Ollama.Model = model
	default:
		if c.Custom == nil {
			c.Custom = make(map[string]ProviderConfig)
		}
		pc := c.Custom[provider]
		pc.Model = model
		// Inherit URL/Env from Registry if not set
		if entry, ok := Registry[provider]; ok {
			if pc.URL == "" {
				pc.URL = entry.DefaultURL
			}
			if pc.Env == "" {
				pc.Env = entry.DefaultEnv
			}
		}
		c.Custom[provider] = pc
	}
}

// Validate checks the config for problems and returns all warnings/errors.
func (c Config) Validate() []string {
	var problems []string

	if c.Provider != "" && c.Provider != "auto" {
		if _, ok := Registry[c.Provider]; !ok {
			if _, ok := c.Custom[c.Provider]; !ok {
				problems = append(problems, fmt.Sprintf("unknown provider %q — add it to [custom.%s] in config.toml or use a known provider", c.Provider, c.Provider))
			}
		}
	}

	for name, pc := range c.Custom {
		if _, ok := Registry[name]; ok {
			continue // registry providers don't need url
		}
		if pc.URL == "" {
			problems = append(problems, fmt.Sprintf("custom provider %q is missing url", name))
		}
		if pc.Env == "" {
			problems = append(problems, fmt.Sprintf("custom provider %q has no env var set (key must be in keyring)", name))
		}
	}

	return problems
}

// CustomEnvs returns a map of provider name to env var name.
// Includes registry providers' env vars, overridden by custom config.
func (c Config) CustomEnvs() map[string]string {
	envs := make(map[string]string)
	for name, entry := range Registry {
		if entry.DefaultEnv != "" {
			envs[name] = entry.DefaultEnv
		}
	}
	for name, pc := range c.Custom {
		if pc.Env != "" {
			envs[name] = pc.Env
		}
	}
	return envs
}
