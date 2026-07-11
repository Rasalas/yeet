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
	cfg.Auto = &config.AutoConfig{Order: []string{"myagent"}}
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
			{name: "bad", model: "bad-model", builder: func() (Provider, error) { return failingProvider{}, nil }},
			{name: "good", model: "good-model", builder: func() (Provider, error) { return fixedProvider{message: "fix: fallback"}, nil }},
		},
	}
	var attempts []ProviderAttempt
	provider.SetAttemptCallback(func(attempt ProviderAttempt) {
		attempts = append(attempts, attempt)
	})

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
	if len(attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(attempts))
	}
	if attempts[1].Previous == nil || attempts[1].Previous.Name != "bad" {
		t.Fatalf("second attempt previous = %#v", attempts[1].Previous)
	}
	if attempts[1].Label() != "good · good-model" {
		t.Fatalf("second attempt label = %q", attempts[1].Label())
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

func TestProviderCommandLineIncludesCodexModelOverride(t *testing.T) {
	rp := config.ResolvedProvider{
		Name:     "codex",
		Model:    "gpt-5.4-mini",
		Command:  "npx",
		Args:     []string{"-y", "@zed-industries/codex-acp@0.16.0"},
		Protocol: config.ProtocolACP,
	}
	got := ProviderCommandLine(rp)
	if !strings.Contains(got, `model="gpt-5.4-mini"`) {
		t.Fatalf("ProviderCommandLine() = %q", got)
	}
	if !strings.Contains(got, `model_reasoning_effort="low"`) {
		t.Fatalf("ProviderCommandLine() = %q", got)
	}
	if !strings.Contains(got, "@zed-industries/codex-acp@0.16.0") {
		t.Fatalf("ProviderCommandLine() = %q", got)
	}
}

func TestProviderCommandLineUsesACPConfigForCurrentCodexAdapter(t *testing.T) {
	rp := config.ResolvedProvider{
		Name:            "codex",
		Model:           "gpt-5.6-luna",
		ReasoningEffort: "low",
		Command:         "npx",
		Args:            []string{"-y", "@agentclientprotocol/codex-acp@1.1.2"},
		Protocol:        config.ProtocolACP,
	}
	got := ProviderCommandLine(rp)
	if strings.Contains(got, `model="`) || strings.Contains(got, "model_reasoning_effort") {
		t.Fatalf("ProviderCommandLine() = %q, current adapter should use ACP config options", got)
	}
}

func TestUsesDefaultACPPackageAllowsPinnedPackage(t *testing.T) {
	if !usesDefaultACPPackage([]string{"-y", "@agentclientprotocol/claude-agent-acp@0.32.0"}, "claude") {
		t.Fatal("expected pinned Claude ACP package to count as default package")
	}
}

func TestUsesDefaultACPPackageAllowsCurrentAndLegacyCodexPackages(t *testing.T) {
	for _, pkg := range []string{"@agentclientprotocol/codex-acp@1.1.2", "@zed-industries/codex-acp@0.16.0"} {
		if !usesDefaultACPPackage([]string{"-y", pkg}, "codex") {
			t.Fatalf("expected %s to count as a default Codex ACP package", pkg)
		}
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
