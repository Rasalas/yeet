package ai

import (
	"fmt"
	"sort"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

// NewProvider creates the appropriate AI provider based on configuration.
func NewProvider(cfg config.Config) (Provider, error) {
	if cfg.Provider == "auto" {
		return autoSelectProvider(cfg)
	}

	rp, ok := cfg.ResolveProviderFull(cfg.Provider)
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s — add it to [custom.%s] in config.toml", cfg.Provider, cfg.Provider)
	}

	return buildProvider(rp)
}

func buildProvider(rp config.ResolvedProvider) (Provider, error) {
	if rp.NeedsAuth {
		key, err := keyring.GetWithEnv(rp.Name, rp.Env)
		if err != nil {
			return nil, fmt.Errorf("%s API key not found — run: yeet auth set %s", rp.Name, rp.Name)
		}
		switch rp.Protocol {
		case config.ProtocolAnthropic:
			return &AnthropicProvider{APIKey: key, Model: rp.Model}, nil
		default:
			return &OpenAIProvider{APIKey: key, Model: rp.Model, BaseURL: rp.URL}, nil
		}
	}

	// No auth required (e.g. Ollama)
	switch rp.Protocol {
	case config.ProtocolOllama:
		return &OllamaProvider{URL: rp.URL, Model: rp.Model}, nil
	default:
		return &OpenAIProvider{Model: rp.Model, BaseURL: rp.URL}, nil
	}
}

type candidate struct {
	model   string
	cost    float64
	builder func() Provider
}

func autoCandidates(cfg config.Config) []candidate {
	var candidates []candidate

	for _, name := range cfg.AllProviders() {
		if name == "auto" {
			continue
		}
		rp, ok := cfg.ResolveProviderFull(name)
		if !ok || rp.Model == "" {
			continue
		}
		// Skip Ollama for auto-select (local, no cost info)
		if rp.Protocol == config.ProtocolOllama {
			continue
		}
		if !rp.NeedsAuth {
			continue
		}
		key, _ := keyring.GetWithEnv(name, rp.Env)
		if key == "" {
			continue
		}

		// Capture for closure
		model, baseURL, proto := rp.Model, rp.URL, rp.Protocol
		candidates = append(candidates, candidate{
			model: model,
			cost:  ModelInputCost(model),
			builder: func() Provider {
				switch proto {
				case config.ProtocolAnthropic:
					return &AnthropicProvider{APIKey: key, Model: model}
				default:
					return &OpenAIProvider{APIKey: key, Model: model, BaseURL: baseURL}
				}
			},
		})
	}

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
