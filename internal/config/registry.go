package config

// Protocol identifies the API protocol a provider uses.
type Protocol string

const (
	ProtocolAnthropic Protocol = "anthropic"
	ProtocolOpenAI    Protocol = "openai"
	ProtocolOllama    Protocol = "ollama"
)

// ProviderEntry holds the static defaults for a known provider.
type ProviderEntry struct {
	DefaultModel string
	DefaultURL   string
	DefaultEnv   string
	Protocol     Protocol
	NeedsAuth    bool
}

// Registry maps provider names to their static defaults.
// It replaces both the builtin hardcoding and the former WellKnown map.
var Registry = map[string]ProviderEntry{
	"anthropic": {
		DefaultModel: "claude-haiku-4-5-20251001",
		DefaultURL:   "https://api.anthropic.com/v1",
		DefaultEnv:   "ANTHROPIC_API_KEY",
		Protocol:     ProtocolAnthropic,
		NeedsAuth:    true,
	},
	"openai": {
		DefaultModel: "gpt-4o-mini",
		DefaultURL:   "https://api.openai.com/v1",
		DefaultEnv:   "OPENAI_API_KEY",
		Protocol:     ProtocolOpenAI,
		NeedsAuth:    true,
	},
	"ollama": {
		DefaultModel: "llama3",
		DefaultURL:   "http://localhost:11434",
		DefaultEnv:   "",
		Protocol:     ProtocolOllama,
		NeedsAuth:    false,
	},
	"google": {
		DefaultModel: "gemini-3-flash-preview",
		DefaultURL:   "https://generativelanguage.googleapis.com/v1beta/openai",
		DefaultEnv:   "GOOGLE_API_KEY",
		Protocol:     ProtocolOpenAI,
		NeedsAuth:    true,
	},
	"groq": {
		DefaultModel: "llama-3.3-70b-versatile",
		DefaultURL:   "https://api.groq.com/openai/v1",
		DefaultEnv:   "GROQ_API_KEY",
		Protocol:     ProtocolOpenAI,
		NeedsAuth:    true,
	},
	"openrouter": {
		DefaultModel: "openrouter/auto",
		DefaultURL:   "https://openrouter.ai/api/v1",
		DefaultEnv:   "OPENROUTER_API_KEY",
		Protocol:     ProtocolOpenAI,
		NeedsAuth:    true,
	},
	"mistral": {
		DefaultModel: "mistral-small-latest",
		DefaultURL:   "https://api.mistral.ai/v1",
		DefaultEnv:   "MISTRAL_API_KEY",
		Protocol:     ProtocolOpenAI,
		NeedsAuth:    true,
	},
}

// ResolvedProvider is the fully-merged provider configuration ready for use.
type ResolvedProvider struct {
	Name      string
	Model     string
	URL       string
	Env       string
	Protocol  Protocol
	NeedsAuth bool
}
