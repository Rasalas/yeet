package ai

import "fmt"

type ModelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// Pricing per 1M tokens (USD). Keep in sync with provider defaults.
var pricing = map[string]ModelPricing{
	// Anthropic
	"claude-haiku-4-5-20251001": {1.00, 5.00},
	"claude-sonnet-4-6":         {3.00, 15.00},
	"claude-opus-4-6":           {5.00, 25.00},

	// OpenAI
	"gpt-4.1-nano": {0.10, 0.40},
	"gpt-4o-mini":  {0.15, 0.60},
	"gpt-4.1-mini": {0.40, 1.60},
	"gpt-4.1":      {2.00, 8.00},
	"gpt-4o":       {2.50, 10.00},
	"o4-mini":      {1.10, 4.40},

	// Google
	"gemini-2.5-flash":      {0.15, 0.60},
	"gemini-3-flash-preview": {0.50, 3.00},

	// Groq
	"llama-3.1-8b-instant":    {0.05, 0.08},
	"llama-3.3-70b-versatile": {0.59, 0.79},
	"openai/gpt-oss-20b":      {0.10, 0.75},

	// Mistral
	"mistral-small-latest": {0.20, 0.60},
	"codestral-latest":     {0.30, 0.90},
	"mistral-large-latest": {0.50, 1.50},
}

// Cost returns the estimated cost in USD and a human-readable string.
// Returns ("", false) if the model has no known pricing (e.g. ollama).
func (u Usage) Cost() (string, bool) {
	p, ok := pricing[u.Model]
	if !ok {
		return "", false
	}
	cost := float64(u.InputTokens)*p.InputPerMillion/1_000_000 +
		float64(u.OutputTokens)*p.OutputPerMillion/1_000_000

	return fmt.Sprintf("$%.4f", cost), true
}

// FormatTokens returns a short token summary like "3.1k in / 28 out".
func (u Usage) FormatTokens() string {
	in := formatCount(u.InputTokens)
	out := formatCount(u.OutputTokens)
	return fmt.Sprintf("%s in / %s out", in, out)
}

func formatCount(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// ModelInputCost returns the input cost per million tokens for a model.
// Returns -1 if the model has no known pricing.
func ModelInputCost(model string) float64 {
	p, ok := pricing[model]
	if !ok {
		return -1
	}
	return p.InputPerMillion
}

// SetPricing adds or overrides pricing for a model.
// Input and output are costs per million tokens in USD.
func SetPricing(model string, input, output float64) {
	pricing[model] = ModelPricing{input, output}
}
