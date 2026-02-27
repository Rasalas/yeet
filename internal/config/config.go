package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type ProviderConfig struct {
	Model string `toml:"model"`
	URL   string `toml:"url,omitempty"`
}

type Config struct {
	Provider  string         `toml:"provider"`
	Anthropic ProviderConfig `toml:"anthropic"`
	OpenAI    ProviderConfig `toml:"openai"`
	Ollama    ProviderConfig `toml:"ollama"`
}

func DefaultConfig() Config {
	return Config{
		Provider:  "anthropic",
		Anthropic: ProviderConfig{Model: "claude-sonnet-4-20250514"},
		OpenAI:    ProviderConfig{Model: "gpt-4o"},
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

func Providers() []string {
	return []string{"anthropic", "openai", "ollama"}
}
