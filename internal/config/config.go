package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/rasalas/yeet/internal/fsutil"
	"github.com/rasalas/yeet/internal/keyring"
	"github.com/rasalas/yeet/internal/xdg"
)

type ProviderConfig struct {
	Model           string   `toml:"model,omitempty"`
	ReasoningEffort string   `toml:"reasoning_effort,omitempty"`
	Upstream        string   `toml:"upstream,omitempty"`
	URL             string   `toml:"url,omitempty"`
	Env             string   `toml:"env,omitempty"`
	Protocol        Protocol `toml:"protocol,omitempty"`
	Command         string   `toml:"command,omitempty"`
	Args            []string `toml:"args,omitempty"`
}

type PricingOverride struct {
	Input  float64 `toml:"input"`
	Output float64 `toml:"output"`
}

type AutoConfig struct {
	Order []string `toml:"order,omitempty"`
}

type Config struct {
	Provider  string                     `toml:"provider"`
	Auto      *AutoConfig                `toml:"auto,omitempty"`
	Anthropic ProviderConfig             `toml:"anthropic"`
	OpenAI    ProviderConfig             `toml:"openai"`
	Ollama    ProviderConfig             `toml:"ollama"`
	Custom    map[string]ProviderConfig  `toml:"custom"`
	Pricing   map[string]PricingOverride `toml:"pricing"`
}

var DefaultAutoOrder = []string{"pi", "codex", "ollama", "claude", "api"}

// KnownModels lists available models per provider for the TUI picker.
var KnownModels = map[string][]string{
	"anthropic":  {"claude-fable-5", "claude-opus-4-8", "claude-sonnet-5", "claude-haiku-4-5-20251001"},
	"openai":     {"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.5-pro", "gpt-5.4", "gpt-5.4-pro", "gpt-5.4-mini", "gpt-5.4-nano", "gpt-5.3-codex", "gpt-5.2", "gpt-4.1-mini"},
	"ollama":     {"llama3", "llama3.1", "gemma2", "mistral", "codellama", "qwen2.5-coder"},
	"codex":      {"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4-mini", "gpt-5.4", "gpt-5.3-codex", "gpt-5.3-codex-spark", "gpt-5.2"},
	"claude":     {"default", "fable", "opus", "sonnet", "haiku", "claude-fable-5", "claude-opus-4-8", "claude-sonnet-5", "claude-haiku-4-5-20251001"},
	"google":     {"gemini-3-flash-preview", "gemini-2.5-flash"},
	"groq":       {"llama-3.3-70b-versatile", "llama-3.1-8b-instant", "openai/gpt-oss-20b"},
	"openrouter": {"openrouter/auto", "google/gemini-3-flash-preview", "openai/gpt-4o-mini"},
	"mistral":    {"mistral-small-latest", "mistral-large-latest", "codestral-latest"},
}

// KnownPiUpstreams is the offline fallback for Pi's dynamic provider list.
// A working Pi installation advertises its configured providers at runtime.
var KnownPiUpstreams = []string{
	"openai-codex",
	"anthropic",
	"github-copilot",
	"google-gemini-cli",
	"google-antigravity",
}

// KnownPiModels provides model-picker fallbacks when Pi model discovery fails.
var KnownPiModels = map[string][]string{
	"openai-codex": {"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4-mini", "gpt-5.4", "gpt-5.3-codex-spark"},
	"anthropic":    {"claude-fable-5", "claude-opus-4-8", "claude-sonnet-5", "claude-haiku-4-5-20251001"},
}

var knownReasoningEfforts = map[string][]string{
	"codex": {"low", "medium", "high", "xhigh"},
	"pi":    {"off", "minimal", "low", "medium", "high", "xhigh", "max"},
}

func ReasoningEffortChoices(provider string) []string {
	choices := knownReasoningEfforts[provider]
	return append([]string(nil), choices...)
}

func SupportsReasoningEffort(provider string) bool {
	return len(knownReasoningEfforts[provider]) > 0
}

func LowestReasoningEffort(provider string) string {
	choices := knownReasoningEfforts[provider]
	if len(choices) == 0 {
		return ""
	}
	return choices[0]
}

func DefaultReasoningEffort(provider string) string {
	if entry, ok := Registry[provider]; ok {
		return entry.DefaultReasoningEffort
	}
	return ""
}

func DefaultUpstream(provider string) string {
	if entry, ok := Registry[provider]; ok {
		return entry.DefaultUpstream
	}
	return ""
}

func ValidReasoningEffort(provider, effort string) bool {
	for _, choice := range knownReasoningEfforts[provider] {
		if effort == choice {
			return true
		}
	}
	return false
}

func (c Config) AutoOrder() []string {
	var order []string
	if c.Auto != nil {
		order = c.Auto.Order
	}
	if len(order) == 0 {
		order = DefaultAutoOrder
	}
	out := make([]string, 0, len(order))
	for _, item := range order {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
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

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(out); err != nil {
		return err
	}

	return fsutil.WriteFileAtomic(path, buf.Bytes(), 0600)
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
		Name:            name,
		Model:           entry.DefaultModel,
		ReasoningEffort: entry.DefaultReasoningEffort,
		Upstream:        entry.DefaultUpstream,
		URL:             entry.DefaultURL,
		Env:             entry.DefaultEnv,
		Command:         entry.DefaultCommand,
		Args:            append([]string(nil), entry.DefaultArgs...),
		Protocol:        entry.Protocol,
		NeedsAuth:       entry.NeedsAuth,
	}

	// Layer 2: named struct fields for builtins
	switch name {
	case "anthropic":
		if c.Anthropic.Model != "" {
			rp.Model = c.Anthropic.Model
		}
		if c.Anthropic.ReasoningEffort != "" {
			rp.ReasoningEffort = c.Anthropic.ReasoningEffort
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
		if c.OpenAI.ReasoningEffort != "" {
			rp.ReasoningEffort = c.OpenAI.ReasoningEffort
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
		if c.Ollama.ReasoningEffort != "" {
			rp.ReasoningEffort = c.Ollama.ReasoningEffort
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
		if custom.ReasoningEffort != "" {
			rp.ReasoningEffort = custom.ReasoningEffort
		}
		if custom.Upstream != "" {
			rp.Upstream = custom.Upstream
		}
		if custom.URL != "" {
			rp.URL = custom.URL
		}
		if custom.Env != "" {
			rp.Env = custom.Env
		}
		if custom.Protocol != "" {
			rp.Protocol = custom.Protocol
		}
		if custom.Command != "" {
			rp.Command = custom.Command
		}
		if custom.Args != nil {
			rp.Args = append([]string(nil), custom.Args...)
		}
	}

	// Purely custom provider not in registry
	if !inRegistry && !hasCustom {
		return ResolvedProvider{}, false
	}
	if !inRegistry {
		if rp.Protocol == "" {
			rp.Protocol = ProtocolOpenAI
		}
		rp.NeedsAuth = rp.Protocol != ProtocolACP && rp.Protocol != ProtocolOllama && rp.Protocol != ProtocolPi
	}
	if rp.Protocol == ProtocolACP || rp.Protocol == ProtocolOllama || rp.Protocol == ProtocolPi {
		rp.NeedsAuth = false
	}

	return rp, true
}

func applyRegistryDefaults(provider string, pc *ProviderConfig) {
	entry, ok := Registry[provider]
	if !ok {
		return
	}
	if pc.URL == "" {
		pc.URL = entry.DefaultURL
	}
	if pc.Env == "" {
		pc.Env = entry.DefaultEnv
	}
	if pc.Protocol == "" {
		pc.Protocol = entry.Protocol
	}
	if pc.Command == "" {
		pc.Command = entry.DefaultCommand
	}
	if pc.Args == nil && entry.DefaultArgs != nil {
		pc.Args = append([]string(nil), entry.DefaultArgs...)
	}
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
		applyRegistryDefaults(provider, &pc)
		c.Custom[provider] = pc
	}
}

// SetUpstream writes the provider selected inside a CLI-backed provider such
// as Pi. The registry default is stored as empty so it can update centrally.
func (c *Config) SetUpstream(provider, upstream string) {
	if upstream == DefaultUpstream(provider) {
		upstream = ""
	}

	pc, exists := c.Custom[provider]
	if upstream == "" && !exists {
		return
	}
	if c.Custom == nil {
		c.Custom = make(map[string]ProviderConfig)
	}
	pc.Upstream = upstream
	applyRegistryDefaults(provider, &pc)
	c.Custom[provider] = pc
}

// SetReasoningEffort writes a reasoning effort override. The provider default is stored as empty.
func (c *Config) SetReasoningEffort(provider, effort string) {
	if effort == DefaultReasoningEffort(provider) {
		effort = ""
	}

	switch provider {
	case "anthropic":
		c.Anthropic.ReasoningEffort = effort
	case "openai":
		c.OpenAI.ReasoningEffort = effort
	case "ollama":
		c.Ollama.ReasoningEffort = effort
	default:
		pc, exists := c.Custom[provider]
		if effort == "" && !exists {
			return
		}
		if c.Custom == nil {
			c.Custom = make(map[string]ProviderConfig)
		}
		pc.ReasoningEffort = effort
		applyRegistryDefaults(provider, &pc)
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

	for _, item := range c.AutoOrder() {
		if item == "api" {
			continue
		}
		if _, ok := c.ResolveProviderFull(item); !ok {
			problems = append(problems, fmt.Sprintf("auto order provider %q is unknown", item))
		}
	}

	for name, pc := range c.Custom {
		if pc.ReasoningEffort != "" && SupportsReasoningEffort(name) && !ValidReasoningEffort(name, pc.ReasoningEffort) {
			problems = append(problems, fmt.Sprintf("custom provider %q has invalid reasoning_effort %q", name, pc.ReasoningEffort))
		}
		if _, ok := Registry[name]; ok {
			continue // registry providers don't need url
		}
		proto := pc.Protocol
		if proto == "" {
			proto = ProtocolOpenAI
		}
		switch proto {
		case ProtocolACP, ProtocolPi:
			if pc.Command == "" {
				problems = append(problems, fmt.Sprintf("custom CLI provider %q is missing command", name))
			}
			if proto == ProtocolPi && pc.Upstream == "" {
				problems = append(problems, fmt.Sprintf("custom Pi provider %q is missing upstream", name))
			}
		case ProtocolOllama:
			if pc.URL == "" {
				problems = append(problems, fmt.Sprintf("custom Ollama provider %q is missing url", name))
			}
		default:
			if pc.URL == "" {
				problems = append(problems, fmt.Sprintf("custom provider %q is missing url", name))
			}
			if pc.Env == "" {
				problems = append(problems, fmt.Sprintf("custom provider %q has no env var set (key must be in keyring)", name))
			}
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
