package ai

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestACPProviderGenerateCommitMessage(t *testing.T) {
	provider := &ACPProvider{
		Name:    "fake",
		Command: os.Args[0],
		Args:    []string{"-test.run=TestFakeACPAgent", "--", "acp-fake"},
	}

	var streamed strings.Builder
	message, usage, err := provider.GenerateCommitMessageStream(CommitContext{
		Diff:   "diff --git a/a.txt b/a.txt\n+hello\n",
		Branch: "fix/acp",
		Status: "M a.txt",
	}, func(token string) {
		streamed.WriteString(token)
	})
	if err != nil {
		t.Fatalf("GenerateCommitMessageStream: %v", err)
	}
	if message != "fix(acp): use fake acp" {
		t.Fatalf("message = %q", message)
	}
	if streamed.String() != "fix(acp): use fake acp" {
		t.Fatalf("streamed = %q", streamed.String())
	}
	if usage.Model != "fake (native config)" {
		t.Fatalf("usage model = %q", usage.Model)
	}
}

func TestACPProviderCodexModelArgs(t *testing.T) {
	provider := &ACPProvider{Name: "codex", Args: []string{"-y", "@zed-industries/codex-acp"}, Model: "gpt-5.4-mini"}
	got := strings.Join(provider.commandArgs(), " ")
	if !strings.Contains(got, `model="gpt-5.4-mini"`) {
		t.Fatalf("commandArgs() = %q", got)
	}
	if !strings.Contains(got, `model_reasoning_effort="low"`) {
		t.Fatalf("commandArgs() = %q", got)
	}
}

func TestACPProviderKeepsExplicitCodexModelArgs(t *testing.T) {
	provider := &ACPProvider{Name: "codex", Args: []string{"-c", `model="gpt-5.5"`}, Model: "gpt-5.4-mini"}
	got := strings.Join(provider.commandArgs(), " ")
	if strings.Contains(got, `model="gpt-5.4-mini"`) {
		t.Fatalf("commandArgs() = %q, should keep explicit model override", got)
	}
	if !strings.Contains(got, `model_reasoning_effort="low"`) {
		t.Fatalf("commandArgs() = %q", got)
	}
}

func TestACPProviderKeepsExplicitCodexReasoningArgs(t *testing.T) {
	provider := &ACPProvider{Name: "codex", Args: []string{"-c", `model_reasoning_effort="high"`}, ReasoningEffort: "low"}
	got := strings.Join(provider.commandArgs(), " ")
	if strings.Count(got, "model_reasoning_effort") != 1 {
		t.Fatalf("commandArgs() = %q, want one reasoning override", got)
	}
	if !strings.Contains(got, `model_reasoning_effort="high"`) {
		t.Fatalf("commandArgs() = %q", got)
	}
}

func TestACPProviderUsesConfiguredCodexReasoning(t *testing.T) {
	provider := &ACPProvider{Name: "codex", ReasoningEffort: "medium"}
	got := strings.Join(provider.commandArgs(), " ")
	if !strings.Contains(got, `model_reasoning_effort="medium"`) {
		t.Fatalf("commandArgs() = %q", got)
	}
}

func TestFakeACPAgent(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "acp-fake" {
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		var msg acpMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			os.Exit(2)
		}
		switch msg.Method {
		case "initialize":
			writeFakeACP(enc, msg.ID, map[string]any{
				"protocolVersion": acpProtocolVersion,
				"agentCapabilities": map[string]any{
					"promptCapabilities": map[string]bool{},
				},
				"authMethods": []any{},
			})
		case "session/new":
			writeFakeACP(enc, msg.ID, map[string]string{"sessionId": "fake-session"})
		case "session/prompt":
			enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      99,
				"method":  "session/request_permission",
				"params": map[string]any{
					"sessionId": "fake-session",
					"toolCall":  map[string]string{"toolCallId": "fake-tool"},
					"options": []map[string]string{
						{"optionId": "allow", "kind": "allow_once"},
						{"optionId": "reject", "kind": "reject_once"},
					},
				},
			})
			if !scanner.Scan() {
				os.Exit(3)
			}
			var permission acpMessage
			if err := json.Unmarshal(scanner.Bytes(), &permission); err != nil {
				os.Exit(4)
			}
			if !fakePermissionRejected(permission.Result) {
				os.Exit(5)
			}
			enc.Encode(map[string]any{
				"jsonrpc": "2.0",
				"method":  "session/update",
				"params": map[string]any{
					"sessionId": "fake-session",
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content": map[string]string{
							"type": "text",
							"text": "fix(acp): use fake acp",
						},
					},
				},
			})
			writeFakeACP(enc, msg.ID, map[string]string{"stopReason": "end_turn"})
			os.Exit(0)
		}
	}
	os.Exit(6)
}

func writeFakeACP(enc *json.Encoder, id *json.RawMessage, result any) {
	_ = enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(*id),
		"result":  result,
	})
}

func fakePermissionRejected(raw json.RawMessage) bool {
	var result struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return false
	}
	return result.Outcome.Outcome == "selected" && result.Outcome.OptionID == "reject"
}
