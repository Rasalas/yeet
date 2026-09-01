package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rasalas/yeet/internal/config"
)

const piCallTimeout = 3 * time.Minute

// PiProvider runs Pi as an ephemeral, tool-free harness. Pi talks to the
// selected model provider directly, so these one-shot requests do not create
// Codex app-server threads or Pi session files.
type PiProvider struct {
	Name            string
	Command         string
	Args            []string
	Model           string
	ReasoningEffort string
}

type piEvent struct {
	Type                  string `json:"type"`
	AssistantMessageEvent struct {
		Type         string `json:"type"`
		Delta        string `json:"delta"`
		ErrorMessage string `json:"errorMessage"`
	} `json:"assistantMessageEvent"`
	Message struct {
		Role         string `json:"role"`
		Model        string `json:"model"`
		StopReason   string `json:"stopReason"`
		ErrorMessage string `json:"errorMessage"`
		Content      []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			Input  int `json:"input"`
			Output int `json:"output"`
		} `json:"usage"`
	} `json:"message"`
}

func (p *PiProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	return p.GenerateCommitMessageStream(ctx, func(string) {})
}

func (p *PiProvider) GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error) {
	if p.Command == "" {
		return "", Usage{}, fmt.Errorf("%s command is not configured", p.Name)
	}

	runCtx, cancel := context.WithTimeout(context.Background(), piCallTimeout)
	defer cancel()

	args := p.commandArgs(ctx.EffectivePrompt())
	cmd := exec.CommandContext(runCtx, p.Command, args...)
	cmd.Stdin = strings.NewReader(buildPiCommitPrompt(ctx))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", Usage{}, err
	}
	var stderr limitedBuffer
	stderr.limit = 64 * 1024
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", Usage{}, fmt.Errorf("failed to start %s (%s): %w", p.Name, piCommandLine(p.Command, p.displayArgs()), err)
	}

	var full strings.Builder
	usage := Usage{Model: p.usageModel("")}
	var streamErr error

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event piEvent
		if err := json.Unmarshal(line, &event); err != nil {
			streamErr = fmt.Errorf("failed to parse Pi output %q: %w", string(line), err)
			break
		}

		if event.Type == "message_update" && event.AssistantMessageEvent.Type == "text_delta" {
			token := event.AssistantMessageEvent.Delta
			full.WriteString(token)
			onToken(token)
		}
		if event.Type == "message_update" && event.AssistantMessageEvent.Type == "error" {
			streamErr = fmt.Errorf("Pi generation failed: %s", strings.TrimSpace(event.AssistantMessageEvent.ErrorMessage))
			break
		}
		if event.Type == "message_end" && event.Message.Role == "assistant" {
			if full.Len() == 0 {
				for _, block := range event.Message.Content {
					if block.Type == "text" && block.Text != "" {
						full.WriteString(block.Text)
						onToken(block.Text)
					}
				}
			}
			usage.Model = p.usageModel(event.Message.Model)
			usage.InputTokens = event.Message.Usage.Input
			usage.OutputTokens = event.Message.Usage.Output
			if event.Message.StopReason == "error" || event.Message.StopReason == "aborted" {
				streamErr = fmt.Errorf("Pi generation %s: %s", event.Message.StopReason, strings.TrimSpace(event.Message.ErrorMessage))
				break
			}
		}
	}
	if err := scanner.Err(); err != nil && streamErr == nil {
		streamErr = err
	}

	if streamErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	if streamErr != nil {
		return "", Usage{}, streamErr
	}
	if runCtx.Err() == context.DeadlineExceeded {
		return "", Usage{}, fmt.Errorf("Pi timed out after %s", piCallTimeout)
	}
	if waitErr != nil {
		if details := stderr.String(); details != "" {
			return "", Usage{}, fmt.Errorf("Pi exited with an error: %s", details)
		}
		return "", Usage{}, fmt.Errorf("Pi exited with an error: %w", waitErr)
	}

	message := cleanACPCommitMessage(full.String())
	if message == "" {
		if details := stderr.String(); details != "" {
			return "", Usage{}, fmt.Errorf("Pi returned an empty message: %s", details)
		}
		return "", Usage{}, fmt.Errorf("Pi returned an empty message")
	}
	return message, usage, nil
}

func (p *PiProvider) commandArgs(systemPrompt string) []string {
	args := p.displayArgs()
	args = append(args,
		"--mode", "json",
		"--no-session",
		"--no-tools",
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-context-files",
		"--system-prompt", systemPrompt,
	)
	return args
}

func (p *PiProvider) displayArgs() []string {
	args := append([]string(nil), p.Args...)
	if !hasCLIFlag(args, "--provider") {
		args = append(args, "--provider", "openai-codex")
	}
	if p.Model != "" && !hasCLIFlag(args, "--model") {
		args = append(args, "--model", p.Model)
	}
	effort := p.ReasoningEffort
	if effort == "" {
		effort = "low"
	}
	if effort != "" && !hasCLIFlag(args, "--thinking") {
		args = append(args, "--thinking", effort)
	}
	return args
}

func hasCLIFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

func (p *PiProvider) usageModel(actual string) string {
	model := strings.TrimSpace(actual)
	if model == "" {
		model = p.Model
	}
	name := p.Name
	if name == "" {
		name = "pi"
	}
	if model == "" {
		return name + " (native config)"
	}
	return name + " · " + model
}

func buildPiCommitPrompt(ctx CommitContext) string {
	return "You are being called by yeet to draft text only.\n" +
		"Do not edit files. Do not run shell commands. Do not inspect the workspace. Use only the context below.\n" +
		"Return exactly the requested message text, with no Markdown, quotes, explanation, or prefix.\n\n" +
		ctx.BuildUserMessage()
}

func piCommandLine(command string, args []string) string {
	return strings.TrimSpace(strings.Join(append([]string{command}, args...), " "))
}

func fetchPiModels(ctx context.Context, rp config.ResolvedProvider) ([]string, error) {
	if rp.Command == "" {
		return nil, fmt.Errorf("%s command is not configured", rp.Name)
	}
	runCtx, cancel := context.WithTimeout(ctx, modelDiscoveryTimeout)
	defer cancel()

	args := append([]string(nil), rp.Args...)
	args = append(args, "--list-models", "openai-codex")
	cmd := exec.CommandContext(runCtx, rp.Command, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("failed to list Pi models: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("failed to list Pi models: %w", err)
	}

	var models []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "openai-codex" {
			continue
		}
		models = appendUnique(models, fields[1])
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Pi did not list any openai-codex models")
	}
	return models, nil
}

func CheckPiProvider(rp config.ResolvedProvider) error {
	ctx, cancel := context.WithTimeout(context.Background(), modelDiscoveryTimeout)
	defer cancel()
	_, err := fetchPiModels(ctx, rp)
	return err
}
