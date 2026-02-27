package ai

import (
	"fmt"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

type Provider interface {
	GenerateCommitMessage(ctx CommitContext) (string, error)
}

func NewProvider(cfg config.Config) (Provider, error) {
	switch cfg.Provider {
	case "anthropic":
		key, err := keyring.Get("anthropic")
		if err != nil {
			return nil, fmt.Errorf("anthropic API key not found — run: yeet auth set anthropic")
		}
		return &AnthropicProvider{APIKey: key, Model: cfg.Anthropic.Model}, nil
	case "openai":
		key, err := keyring.Get("openai")
		if err != nil {
			return nil, fmt.Errorf("openai API key not found — run: yeet auth set openai")
		}
		return &OpenAIProvider{APIKey: key, Model: cfg.OpenAI.Model}, nil
	case "ollama":
		url := cfg.Ollama.URL
		if url == "" {
			url = "http://localhost:11434"
		}
		return &OllamaProvider{URL: url, Model: cfg.Ollama.Model}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}
