package ai

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rasalas/yeet/internal/config"
)

func TestAutoProviderPrefersAvailableLocalProvider(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Ollama.URL = "http://127.0.0.1:1"
	cfg.Custom = map[string]config.ProviderConfig{
		"codex":   {Protocol: config.ProtocolACP, Command: "definitely-not-a-command"},
		"claude":  {Protocol: config.ProtocolACP, Command: "definitely-not-a-command"},
		"myagent": {Protocol: config.ProtocolACP, Command: os.Args[0]},
	}

	if got := AutoProviderName(cfg); got != "myagent" {
		t.Fatalf("AutoProviderName() = %q, want myagent", got)
	}
	if got := AutoModelName(cfg); got != "myagent (native CLI config)" {
		t.Fatalf("AutoModelName() = %q", got)
	}
}

func TestAutoProviderFallsBackAfterFailure(t *testing.T) {
	provider := &AutoProvider{
		candidates: []candidate{
			{name: "bad", builder: func() (Provider, error) { return failingProvider{}, nil }},
			{name: "good", builder: func() (Provider, error) { return fixedProvider{message: "fix: fallback"}, nil }},
		},
	}

	msg, usage, err := provider.GenerateCommitMessage(CommitContext{})
	if err != nil {
		t.Fatalf("GenerateCommitMessage: %v", err)
	}
	if msg != "fix: fallback" {
		t.Fatalf("message = %q", msg)
	}
	if usage.Model != "good" {
		t.Fatalf("usage model = %q", usage.Model)
	}
}

func TestAutoProviderStreamsFallback(t *testing.T) {
	provider := &AutoProvider{
		candidates: []candidate{
			{name: "bad", builder: func() (Provider, error) { return failingStreamingProvider{}, nil }},
			{name: "good", builder: func() (Provider, error) { return fixedProvider{message: "fix: stream fallback"}, nil }},
		},
	}

	var streamed strings.Builder
	msg, _, err := provider.GenerateCommitMessageStream(CommitContext{}, func(token string) {
		streamed.WriteString(token)
	})
	if err != nil {
		t.Fatalf("GenerateCommitMessageStream: %v", err)
	}
	if msg != "fix: stream fallback" {
		t.Fatalf("message = %q", msg)
	}
	if streamed.String() != "fix: stream fallback" {
		t.Fatalf("streamed = %q", streamed.String())
	}
}

type failingProvider struct{}

func (failingProvider) GenerateCommitMessage(CommitContext) (string, Usage, error) {
	return "", Usage{}, fmt.Errorf("boom")
}

type failingStreamingProvider struct{}

func (failingStreamingProvider) GenerateCommitMessage(CommitContext) (string, Usage, error) {
	return "", Usage{}, fmt.Errorf("boom")
}

func (failingStreamingProvider) GenerateCommitMessageStream(CommitContext, func(string)) (string, Usage, error) {
	return "", Usage{}, fmt.Errorf("boom")
}

type fixedProvider struct {
	message string
}

func (p fixedProvider) GenerateCommitMessage(CommitContext) (string, Usage, error) {
	return p.message, Usage{Model: "good"}, nil
}
