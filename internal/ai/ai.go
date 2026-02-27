package ai

import (
	"fmt"
	"sort"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

type Usage struct {
	Model        string
	InputTokens  int
	OutputTokens int
}

type Provider interface {
	GenerateCommitMessage(ctx CommitContext) (string, Usage, error)
}

func NewProvider(cfg config.Config) (Provider, error) {
	switch cfg.Provider {
	case "auto":
		return autoSelectProvider(cfg)
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
		return &OpenAIProvider{APIKey: key, Model: cfg.OpenAI.Model, BaseURL: cfg.OpenAI.URL}, nil
	case "ollama":
		url := cfg.Ollama.URL
		if url == "" {
			url = config.DefaultOllamaURL
		}
		return &OllamaProvider{URL: url, Model: cfg.Ollama.Model}, nil
	default:
		pc, ok := cfg.ResolveProvider(cfg.Provider)
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s — add it to [custom.%s] in config.toml", cfg.Provider, cfg.Provider)
		}
		key, err := keyring.GetWithEnv(cfg.Provider, pc.Env)
		if err != nil {
			return nil, fmt.Errorf("%s API key not found — run: yeet auth set %s", cfg.Provider, cfg.Provider)
		}
		return &OpenAIProvider{APIKey: key, Model: pc.Model, BaseURL: pc.URL}, nil
	}
}

type candidate struct {
	model   string
	cost    float64
	builder func() Provider
}

func autoCandidates(cfg config.Config) []candidate {
	var candidates []candidate

	// Anthropic
	if key, _ := keyring.Get("anthropic"); key != "" {
		model := cfg.Anthropic.Model
		candidates = append(candidates, candidate{
			model: model,
			cost:  ModelInputCost(model),
			builder: func() Provider {
				return &AnthropicProvider{APIKey: key, Model: model}
			},
		})
	}

	// OpenAI
	if key, _ := keyring.Get("openai"); key != "" {
		model, baseURL := cfg.OpenAI.Model, cfg.OpenAI.URL
		candidates = append(candidates, candidate{
			model: model,
			cost:  ModelInputCost(model),
			builder: func() Provider {
				return &OpenAIProvider{APIKey: key, Model: model, BaseURL: baseURL}
			},
		})
	}

	// Non-builtin providers (custom config + well-known)
	seen := map[string]bool{"anthropic": true, "openai": true, "ollama": true}
	for _, name := range cfg.AllProviders() {
		if seen[name] {
			continue
		}
		seen[name] = true
		pc, ok := cfg.ResolveProvider(name)
		if !ok || pc.Model == "" || pc.URL == "" {
			continue
		}
		if key, _ := keyring.GetWithEnv(name, pc.Env); key != "" {
			model, baseURL := pc.Model, pc.URL
			candidates = append(candidates, candidate{
				model: model,
				cost:  ModelInputCost(model),
				builder: func() Provider {
					return &OpenAIProvider{APIKey: key, Model: model, BaseURL: baseURL}
				},
			})
		}
	}

	// Sort by cost (cheapest first), unknown pricing (-1) at end
	sort.Slice(candidates, func(i, j int) bool {
		ci, cj := candidates[i].cost, candidates[j].cost
		if ci < 0 {
			return false
		}
		if cj < 0 {
			return true
		}
		return ci < cj
	})

	return candidates
}

// autoSelectProvider picks the cheapest available cloud provider.
func autoSelectProvider(cfg config.Config) (Provider, error) {
	candidates := autoCandidates(cfg)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no API key found for any provider — run: yeet auth set <provider>")
	}
	return candidates[0].builder(), nil
}

// AutoModelName returns the model name that "auto" would currently select,
// or "" if no provider is available.
func AutoModelName(cfg config.Config) string {
	candidates := autoCandidates(cfg)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].model
}
