package ai

import (
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/rasalas/yeet/internal/config"
	"github.com/rasalas/yeet/internal/keyring"
)

var autoLocalOrder = []string{"codex", "ollama", "claude"}

var autoOllamaClient = &http.Client{Timeout: 500 * time.Millisecond}

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
	if rp.Protocol == config.ProtocolACP {
		if rp.Command == "" {
			return nil, fmt.Errorf("%s ACP command is not configured", rp.Name)
		}
		return &ACPProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args, Model: rp.Model}, nil
	}

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
	name    string
	model   string
	cost    float64
	builder func() (Provider, error)
}

func autoCandidates(cfg config.Config) []candidate {
	return append(autoLocalCandidates(cfg), autoAPICandidates(cfg)...)
}

func autoLocalCandidates(cfg config.Config) []candidate {
	var candidates []candidate
	seen := make(map[string]bool)

	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true

		rp, ok := cfg.ResolveProviderFull(name)
		if !ok {
			return
		}
		switch rp.Protocol {
		case config.ProtocolACP:
			if rp.Command == "" {
				return
			}
			if _, err := exec.LookPath(rp.Command); err != nil {
				return
			}
		case config.ProtocolOllama:
			if !ollamaAvailable(rp.URL) {
				return
			}
		default:
			return
		}

		resolved := rp
		candidates = append(candidates, candidate{
			name:  resolved.Name,
			model: autoDisplayModel(resolved),
			cost:  -1,
			builder: func() (Provider, error) {
				return buildProvider(resolved)
			},
		})
	}

	for _, name := range autoLocalOrder {
		add(name)
	}
	for _, name := range cfg.AllProviders() {
		add(name)
	}

	return candidates
}

func autoAPICandidates(cfg config.Config) []candidate {
	var candidates []candidate

	for _, name := range cfg.AllProviders() {
		if name == "auto" {
			continue
		}
		rp, ok := cfg.ResolveProviderFull(name)
		if !ok || rp.Model == "" {
			continue
		}
		// Skip local/agent providers for auto-select (no comparable cost info).
		if rp.Protocol == config.ProtocolOllama || rp.Protocol == config.ProtocolACP {
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
		model, baseURL, proto, providerName := rp.Model, rp.URL, rp.Protocol, rp.Name
		candidates = append(candidates, candidate{
			name:  providerName,
			model: model,
			cost:  ModelInputCost(model),
			builder: func() (Provider, error) {
				switch proto {
				case config.ProtocolAnthropic:
					return &AnthropicProvider{APIKey: key, Model: model}, nil
				default:
					return &OpenAIProvider{APIKey: key, Model: model, BaseURL: baseURL}, nil
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

// autoSelectProvider prefers local/native providers, then falls back to the
// cheapest configured cloud provider.
func autoSelectProvider(cfg config.Config) (Provider, error) {
	candidates := autoCandidates(cfg)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no local provider or API key found — run: yeet config")
	}
	return &AutoProvider{candidates: candidates}, nil
}

// AutoProviderName returns the provider name that "auto" would try first.
func AutoProviderName(cfg config.Config) string {
	candidates := autoCandidates(cfg)
	if len(candidates) == 0 {
		return ""
	}
	return candidates[0].name
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

type AutoProvider struct {
	candidates []candidate
}

func (p *AutoProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	var failures []string
	for _, c := range p.candidates {
		provider, err := c.builder()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
			continue
		}
		msg, usage, err := provider.GenerateCommitMessage(ctx)
		if err == nil {
			return msg, usage, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
	}
	return "", Usage{}, fmt.Errorf("auto providers failed: %s", strings.Join(failures, "; "))
}

func (p *AutoProvider) GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error) {
	var failures []string
	for _, c := range p.candidates {
		provider, err := c.builder()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
			continue
		}

		if sp, ok := provider.(StreamingProvider); ok {
			streamed := false
			msg, usage, err := sp.GenerateCommitMessageStream(ctx, func(token string) {
				streamed = true
				onToken(token)
			})
			if err == nil {
				return msg, usage, nil
			}
			failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
			if streamed {
				return "", Usage{}, fmt.Errorf("auto provider %s failed after streaming started: %w", c.name, err)
			}
			continue
		}

		msg, usage, err := provider.GenerateCommitMessage(ctx)
		if err == nil {
			onToken(msg)
			return msg, usage, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
	}
	return "", Usage{}, fmt.Errorf("auto providers failed: %s", strings.Join(failures, "; "))
}

func autoDisplayModel(rp config.ResolvedProvider) string {
	if rp.Protocol == config.ProtocolACP {
		if rp.Model != "" {
			return rp.Name + " · " + rp.Model
		}
		return rp.Name + " (native CLI config)"
	}
	if rp.Model != "" {
		return rp.Model
	}
	return rp.Name
}

func ollamaAvailable(baseURL string) bool {
	if baseURL == "" {
		return false
	}
	url := strings.TrimRight(baseURL, "/") + "/api/tags"
	resp, err := autoOllamaClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
