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
		rp = resolveACPProvider(rp)
		if rp.Command == "" {
			return nil, fmt.Errorf("%s ACP command is not configured", rp.Name)
		}
		return &ACPProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args, Model: rp.Model, ReasoningEffort: rp.ReasoningEffort}, nil
	}
	if rp.Protocol == config.ProtocolPi {
		if rp.Command == "" {
			return nil, fmt.Errorf("%s command is not configured", rp.Name)
		}
		return &PiProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args, Upstream: rp.Upstream, Model: rp.Model, ReasoningEffort: rp.ReasoningEffort}, nil
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
	var candidates []candidate
	seen := make(map[string]bool)
	apiCandidates := autoAPICandidates(cfg)

	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true

		c, ok := autoCandidateForProvider(cfg, name)
		if !ok {
			return
		}
		candidates = append(candidates, c)
	}

	addAPI := func() {
		for _, c := range apiCandidates {
			if seen[c.name] {
				continue
			}
			seen[c.name] = true
			candidates = append(candidates, c)
		}
	}

	for _, item := range cfg.AutoOrder() {
		if item == "api" {
			addAPI()
			continue
		}
		add(item)
	}

	return candidates
}

func autoCandidateForProvider(cfg config.Config, name string) (candidate, bool) {
	rp, ok := cfg.ResolveProviderFull(name)
	if !ok || rp.Model == "" && rp.Protocol != config.ProtocolACP && rp.Protocol != config.ProtocolPi {
		return candidate{}, false
	}

	switch rp.Protocol {
	case config.ProtocolACP:
		rp = resolveACPProvider(rp)
		if rp.Command == "" {
			return candidate{}, false
		}
		if _, err := exec.LookPath(rp.Command); err != nil {
			return candidate{}, false
		}
		resolved := rp
		return candidate{
			name:  resolved.Name,
			model: autoDisplayModel(resolved),
			cost:  -1,
			builder: func() (Provider, error) {
				return buildProvider(resolved)
			},
		}, true
	case config.ProtocolPi:
		if rp.Command == "" {
			return candidate{}, false
		}
		if _, err := exec.LookPath(rp.Command); err != nil {
			return candidate{}, false
		}
		resolved := rp
		return candidate{
			name:  resolved.Name,
			model: autoDisplayModel(resolved),
			cost:  -1,
			builder: func() (Provider, error) {
				return buildProvider(resolved)
			},
		}, true
	case config.ProtocolOllama:
		if !ollamaAvailable(rp.URL) {
			return candidate{}, false
		}
		resolved := rp
		return candidate{
			name:  resolved.Name,
			model: autoDisplayModel(resolved),
			cost:  -1,
			builder: func() (Provider, error) {
				return buildProvider(resolved)
			},
		}, true
	default:
		if !rp.NeedsAuth {
			return candidate{}, false
		}
		key, _ := keyring.GetWithEnv(name, rp.Env)
		if key == "" {
			return candidate{}, false
		}
		return apiCandidate(rp, key), true
	}
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
		if rp.Protocol == config.ProtocolOllama || rp.Protocol == config.ProtocolACP || rp.Protocol == config.ProtocolPi {
			continue
		}
		if !rp.NeedsAuth {
			continue
		}
		key, _ := keyring.GetWithEnv(name, rp.Env)
		if key == "" {
			continue
		}

		candidates = append(candidates, apiCandidate(rp, key))
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

func apiCandidate(rp config.ResolvedProvider, key string) candidate {
	model, baseURL, proto, providerName := rp.Model, rp.URL, rp.Protocol, rp.Name
	return candidate{
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
	}
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
	onAttempt  func(ProviderAttempt)
}

func (p *AutoProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	var failures []string
	var previous *ProviderFailure
	for _, c := range p.candidates {
		p.notifyAttempt(c, previous)
		provider, err := c.builder()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
			previous = &ProviderFailure{Name: c.name, Err: err}
			continue
		}
		msg, usage, err := provider.GenerateCommitMessage(ctx)
		if err == nil {
			return msg, usage, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
		previous = &ProviderFailure{Name: c.name, Err: err}
	}
	return "", Usage{}, fmt.Errorf("auto providers failed: %s", strings.Join(failures, "; "))
}

func (p *AutoProvider) GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error) {
	var failures []string
	var previous *ProviderFailure
	for _, c := range p.candidates {
		p.notifyAttempt(c, previous)
		provider, err := c.builder()
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
			previous = &ProviderFailure{Name: c.name, Err: err}
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
			previous = &ProviderFailure{Name: c.name, Err: err}
			continue
		}

		msg, usage, err := provider.GenerateCommitMessage(ctx)
		if err == nil {
			onToken(msg)
			return msg, usage, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", c.name, err))
		previous = &ProviderFailure{Name: c.name, Err: err}
	}
	return "", Usage{}, fmt.Errorf("auto providers failed: %s", strings.Join(failures, "; "))
}

type ProviderFailure struct {
	Name string
	Err  error
}

type ProviderAttempt struct {
	Name     string
	Model    string
	Previous *ProviderFailure
}

func (a ProviderAttempt) Label() string {
	return providerLabel(a.Name, a.Model)
}

type AttemptReporter interface {
	SetAttemptCallback(func(ProviderAttempt))
}

func (p *AutoProvider) SetAttemptCallback(callback func(ProviderAttempt)) {
	p.onAttempt = callback
}

func (p *AutoProvider) notifyAttempt(c candidate, previous *ProviderFailure) {
	if p.onAttempt == nil {
		return
	}
	p.onAttempt(ProviderAttempt{Name: c.name, Model: c.model, Previous: previous})
}

func ConfiguredProviderLabel(cfg config.Config) string {
	if cfg.Provider == "auto" {
		candidates := autoCandidates(cfg)
		if len(candidates) == 0 {
			return "auto"
		}
		return candidates[0].label()
	}
	rp, ok := cfg.ResolveProviderFull(cfg.Provider)
	if !ok {
		return cfg.Provider
	}
	return providerLabel(rp.Name, autoDisplayModel(rp))
}

func (c candidate) label() string {
	return providerLabel(c.name, c.model)
}

func providerLabel(name, model string) string {
	if model == "" || model == name {
		return name
	}
	if strings.HasPrefix(model, name+" ·") || strings.HasPrefix(model, name+" (") {
		return model
	}
	return name + " · " + model
}

func autoDisplayModel(rp config.ResolvedProvider) string {
	if rp.Protocol == config.ProtocolPi {
		upstream := rp.Upstream
		if upstream == "" {
			upstream = "native provider"
		}
		if rp.Model != "" {
			return rp.Name + " · " + upstream + " · " + rp.Model
		}
		return rp.Name + " · " + upstream + " (native model)"
	}
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

func resolveACPProvider(rp config.ResolvedProvider) config.ResolvedProvider {
	if rp.Protocol != config.ProtocolACP || rp.Command != "npx" {
		return rp
	}
	binary, ok := localACPBinary(rp.Name)
	if !ok {
		return rp
	}
	if !usesDefaultACPPackage(rp.Args, rp.Name) {
		return rp
	}
	if _, err := exec.LookPath(binary); err != nil {
		return rp
	}
	rp.Command = binary
	rp.Args = nil
	return rp
}

func ProviderCommandLine(rp config.ResolvedProvider) string {
	if rp.Protocol == config.ProtocolACP {
		rp = resolveACPProvider(rp)
		return acpCommandLine(rp.Command, (&ACPProvider{Name: rp.Name, Args: rp.Args, Model: rp.Model, ReasoningEffort: rp.ReasoningEffort}).commandArgs())
	}
	if rp.Protocol == config.ProtocolPi {
		provider := &PiProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args, Upstream: rp.Upstream, Model: rp.Model, ReasoningEffort: rp.ReasoningEffort}
		return strings.Join(append([]string{rp.Command}, provider.displayArgs()...), " ")
	}
	return strings.TrimSpace(strings.Join(append([]string{rp.Command}, rp.Args...), " "))
}

func localACPBinary(provider string) (string, bool) {
	switch provider {
	case "codex":
		return "codex-acp", true
	case "claude":
		return "claude-agent-acp", true
	default:
		return "", false
	}
}

func usesDefaultACPPackage(args []string, provider string) bool {
	var wants []string
	switch provider {
	case "codex":
		wants = []string{"@agentclientprotocol/codex-acp", "@zed-industries/codex-acp"}
	case "claude":
		wants = []string{"@agentclientprotocol/claude-agent-acp"}
	default:
		return false
	}
	for _, arg := range args {
		for _, want := range wants {
			if strings.HasPrefix(arg, want) {
				return true
			}
		}
	}
	return false
}
