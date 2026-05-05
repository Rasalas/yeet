package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rasalas/yeet/internal/config"
)

const acpProtocolVersion = 1
const acpCallTimeout = 3 * time.Minute

type ACPProvider struct {
	Name    string
	Command string
	Args    []string
	Model   string
}

type acpConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	enc    *json.Encoder
	mu     sync.Mutex
	lines  chan acpReadResult
	stderr *limitedBuffer
}

type acpReadResult struct {
	msg acpMessage
	err error
}

type acpMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *acpError        `json:"error,omitempty"`
}

type acpError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type limitedBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.buf.Len() < b.limit {
		remaining := b.limit - b.buf.Len()
		if len(p) > remaining {
			b.buf.Write(p[:remaining])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.buf.String())
}

func (p *ACPProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	return p.GenerateCommitMessageStream(ctx, func(string) {})
}

func (p *ACPProvider) GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error) {
	if p.Command == "" {
		return "", Usage{}, fmt.Errorf("%s ACP command is not configured", p.Name)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", Usage{}, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	args := p.commandArgs()
	conn, err := startACP(runCtx, p.Command, args, cwd)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to start %s ACP agent (%s): %w", p.Name, acpCommandLine(p.Command, args), err)
	}
	defer conn.close()

	if _, err := conn.call(1, "initialize", map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientCapabilities": map[string]any{
			"fs": map[string]any{
				"readTextFile":  false,
				"writeTextFile": false,
			},
			"terminal": false,
		},
		"clientInfo": map[string]string{
			"name":    "yeet",
			"title":   "yeet",
			"version": "0.0.0",
		},
	}, nil); err != nil {
		return "", Usage{}, err
	}

	result, err := conn.call(2, "session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
		"_meta":      acpSessionMeta(ctx.EffectivePrompt(), p.Model),
	}, nil)
	if err != nil {
		return "", Usage{}, err
	}

	var session struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(result, &session); err != nil {
		return "", Usage{}, fmt.Errorf("failed to parse ACP session response: %w", err)
	}
	if session.SessionID == "" {
		return "", Usage{}, fmt.Errorf("ACP agent did not return a session id")
	}

	var full strings.Builder
	_, err = conn.call(3, "session/prompt", map[string]any{
		"sessionId": session.SessionID,
		"prompt": []map[string]string{
			{
				"type": "text",
				"text": buildACPCommitPrompt(ctx),
			},
		},
	}, func(msg acpMessage) error {
		token, ok := acpAgentTextChunk(msg)
		if !ok {
			return nil
		}
		full.WriteString(token)
		onToken(token)
		return nil
	})
	if err != nil {
		return "", Usage{}, err
	}

	message := cleanACPCommitMessage(full.String())
	if message == "" {
		return "", Usage{}, fmt.Errorf("ACP agent returned an empty message")
	}

	return message, Usage{Model: p.usageModel()}, nil
}

func startACP(ctx context.Context, command string, args []string, cwd string) (*acpConn, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = cwd

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	conn := &acpConn{
		cmd:    cmd,
		stdin:  stdin,
		enc:    json.NewEncoder(stdin),
		lines:  make(chan acpReadResult, 64),
		stderr: &limitedBuffer{limit: 64 * 1024},
	}

	go func() {
		_, _ = io.Copy(conn.stderr, stderr)
	}()
	go conn.readStdout(stdout)

	return conn, nil
}

func CheckACPProvider(rp config.ResolvedProvider) error {
	rp = resolveACPProvider(rp)
	if rp.Command == "" {
		return fmt.Errorf("%s ACP command is not configured", rp.Name)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provider := &ACPProvider{Name: rp.Name, Command: rp.Command, Args: rp.Args, Model: rp.Model}
	conn, err := startACP(runCtx, provider.Command, provider.commandArgs(), cwd)
	if err != nil {
		return fmt.Errorf("failed to start %s ACP agent (%s): %w", rp.Name, ProviderCommandLine(rp), err)
	}
	defer conn.close()

	if _, err := conn.call(1, "initialize", map[string]any{
		"protocolVersion": acpProtocolVersion,
		"clientCapabilities": map[string]any{
			"fs":       map[string]bool{"readTextFile": false, "writeTextFile": false},
			"terminal": false,
		},
		"clientInfo": map[string]string{
			"name":    "yeet",
			"title":   "yeet",
			"version": "0.0.0",
		},
	}, nil); err != nil {
		return err
	}

	_, err = conn.call(2, "session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": []any{},
		"_meta":      acpSessionMeta("Smoke test only. Do not generate text.", rp.Model),
	}, nil)
	return err
}

func acpSessionMeta(systemPrompt, model string) map[string]any {
	claudeOptions := map[string]any{
		"tools": []any{},
	}
	if model != "" {
		claudeOptions["model"] = model
	}
	meta := map[string]any{
		"disableBuiltInTools": true,
		"systemPrompt":        systemPrompt,
		"claudeCode": map[string]any{
			"options": claudeOptions,
		},
	}
	if model != "" {
		meta["model"] = model
	}
	return meta
}

func (c *acpConn) readStdout(stdout io.Reader) {
	defer close(c.lines)
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg acpMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			c.lines <- acpReadResult{err: fmt.Errorf("failed to parse ACP message %q: %w", string(line), err)}
			continue
		}
		c.lines <- acpReadResult{msg: msg}
	}
	if err := scanner.Err(); err != nil {
		c.lines <- acpReadResult{err: err}
	}
}

func (c *acpConn) call(id int64, method string, params any, onUpdate func(acpMessage) error) (json.RawMessage, error) {
	if err := c.send(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}); err != nil {
		return nil, err
	}

	timer := time.NewTimer(acpCallTimeout)
	defer timer.Stop()

	for {
		select {
		case item, ok := <-c.lines:
			if !ok {
				if stderr := c.stderr.String(); stderr != "" {
					return nil, fmt.Errorf("ACP agent exited before %s completed: %s", method, stderr)
				}
				return nil, fmt.Errorf("ACP agent exited before %s completed", method)
			}
			if item.err != nil {
				return nil, item.err
			}
			msg := item.msg
			if msg.Method != "" {
				if err := c.handleAgentMessage(msg, onUpdate); err != nil {
					return nil, err
				}
				continue
			}
			if !acpIDMatches(msg.ID, id) {
				continue
			}
			if msg.Error != nil {
				return nil, c.formatACPError(method, msg.Error)
			}
			return msg.Result, nil
		case <-timer.C:
			return nil, fmt.Errorf("ACP agent timed out waiting for %s", method)
		}
	}
}

func (c *acpConn) send(msg any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.enc.Encode(msg)
}

func (c *acpConn) handleAgentMessage(msg acpMessage, onUpdate func(acpMessage) error) error {
	if msg.ID == nil {
		if msg.Method == "session/update" && onUpdate != nil {
			return onUpdate(msg)
		}
		return nil
	}

	switch msg.Method {
	case "session/request_permission":
		return c.sendResult(msg.ID, acpRejectPermission(msg.Params))
	default:
		return c.sendError(msg.ID, -32601, "method not available")
	}
}

func (c *acpConn) sendResult(id *json.RawMessage, result any) error {
	return c.send(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(*id),
		"result":  result,
	})
}

func (c *acpConn) sendError(id *json.RawMessage, code int, message string) error {
	return c.send(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(*id),
		"error": acpError{
			Code:    code,
			Message: message,
		},
	})
}

func (c *acpConn) formatACPError(method string, err *acpError) error {
	msg := strings.TrimSpace(err.Message)
	if msg == "" {
		msg = fmt.Sprintf("ACP error %d", err.Code)
	}
	if stderr := c.stderr.String(); stderr != "" {
		return fmt.Errorf("%s failed: %s (%s)", method, msg, stderr)
	}
	return fmt.Errorf("%s failed: %s", method, msg)
}

func (c *acpConn) close() {
	_ = c.stdin.Close()
	done := make(chan struct{})
	go func() {
		_ = c.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		<-done
	}
}

func acpIDMatches(raw *json.RawMessage, want int64) bool {
	if raw == nil {
		return false
	}
	var n int64
	if err := json.Unmarshal(*raw, &n); err == nil {
		return n == want
	}
	var s string
	if err := json.Unmarshal(*raw, &s); err == nil {
		return s == strconv.FormatInt(want, 10)
	}
	return false
}

func acpAgentTextChunk(msg acpMessage) (string, bool) {
	if msg.Method != "session/update" {
		return "", false
	}
	var params struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			Content       struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return "", false
	}
	if params.Update.SessionUpdate != "agent_message_chunk" || params.Update.Content.Type != "text" {
		return "", false
	}
	return params.Update.Content.Text, params.Update.Content.Text != ""
}

func acpRejectPermission(params json.RawMessage) map[string]any {
	var req struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	if err := json.Unmarshal(params, &req); err == nil {
		for _, opt := range req.Options {
			if strings.HasPrefix(opt.Kind, "reject") && opt.OptionID != "" {
				return map[string]any{
					"outcome": map[string]any{
						"outcome":  "selected",
						"optionId": opt.OptionID,
					},
				}
			}
		}
	}
	return map[string]any{
		"outcome": map[string]any{
			"outcome": "cancelled",
		},
	}
}

func buildACPCommitPrompt(ctx CommitContext) string {
	var b strings.Builder
	b.WriteString("You are being called by yeet to draft text only.\n")
	b.WriteString("Do not edit files. Do not run shell commands. Do not inspect the workspace. Use only the context below.\n")
	b.WriteString("Return exactly the requested message text, with no Markdown, quotes, explanation, or prefix.\n\n")
	b.WriteString(ctx.EffectivePrompt())
	b.WriteString("\n\n")
	b.WriteString(ctx.BuildUserMessage())
	return b.String()
}

func cleanACPCommitMessage(message string) string {
	message = strings.TrimSpace(message)
	if strings.HasPrefix(message, "```") {
		lines := strings.Split(message, "\n")
		if len(lines) > 2 {
			message = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	message = strings.TrimSpace(message)
	message = strings.Trim(message, "`")
	message = strings.TrimSpace(message)
	message = strings.Trim(message, `"'`)
	return strings.TrimSpace(message)
}

func (p *ACPProvider) usageModel() string {
	if p.Model != "" {
		if p.Name != "" {
			return p.Name + " · " + p.Model
		}
		return p.Model
	}
	if p.Name != "" {
		return p.Name + " (native config)"
	}
	return "acp"
}

func (p *ACPProvider) commandArgs() []string {
	args := append([]string(nil), p.Args...)
	if p.Model == "" || p.Name != "codex" || hasCodexModelOverride(args) {
		return args
	}
	return append(args, "-c", fmt.Sprintf("model=%q", p.Model))
}

func acpCommandLine(command string, args []string) string {
	parts := append([]string{command}, args...)
	return strings.Join(parts, " ")
}

func hasCodexModelOverride(args []string) bool {
	for i, arg := range args {
		if arg == "-m" || arg == "--model" {
			return true
		}
		if (arg == "-c" || arg == "--config") && i+1 < len(args) && strings.HasPrefix(args[i+1], "model=") {
			return true
		}
		if strings.HasPrefix(arg, "--model=") || strings.HasPrefix(arg, "-m=") {
			return true
		}
		if strings.HasPrefix(arg, "--config=model=") || strings.HasPrefix(arg, "-c=model=") || strings.HasPrefix(arg, "model=") {
			return true
		}
	}
	return false
}
