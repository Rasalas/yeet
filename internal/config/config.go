package config

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/rasalas/yeet/internal/keyring"
)

type ProviderConfig struct {
	Model string `toml:"model"`
	URL   string `toml:"url,omitempty"`
	Env   string `toml:"env,omitempty"`
}

type Config struct {
	Provider  string                    `toml:"provider"`
	Anthropic ProviderConfig            `toml:"anthropic"`
	OpenAI    ProviderConfig            `toml:"openai"`
	Ollama    ProviderConfig            `toml:"ollama"`
	Custom    map[string]ProviderConfig `toml:"custom"`
}

func DefaultConfig() Config {
	return Config{
		Provider:  "auto",
		Anthropic: ProviderConfig{Model: "claude-haiku-4-5-20251001"},
		OpenAI:    ProviderConfig{Model: "gpt-4o-mini"},
		Ollama:    ProviderConfig{Model: "llama3", URL: "http://localhost:11434"},
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "yeet", "config.toml"), nil
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
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
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

// CustomEnvs returns a map of provider name to custom env var name.
func (c Config) CustomEnvs() map[string]string {
	envs := make(map[string]string)
	for name, pc := range c.Custom {
		if pc.Env != "" {
			envs[name] = pc.Env
		}
	}
	return envs
}
