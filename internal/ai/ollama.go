package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OllamaProvider struct {
	URL   string
	Model string
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	Error           string `json:"error,omitempty"`
}

func (p *OllamaProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	body := ollamaRequest{
		Model: p.Model,
		Messages: []ollamaMessage{
			{Role: "system", Content: LoadPrompt()},
			{Role: "user", Content: ctx.BuildUserMessage()},
		},
		Stream: false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := strings.TrimRight(p.URL, "/") + "/api/chat"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("API request failed (is Ollama running at %s?): %w", p.URL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to read response: %w", err)
	}

	var result ollamaResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", Usage{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != "" {
		return "", Usage{}, fmt.Errorf("Ollama error: %s", result.Error)
	}

	usage := Usage{
		Model:        p.Model,
		InputTokens:  result.PromptEvalCount,
		OutputTokens: result.EvalCount,
	}

	return strings.TrimSpace(result.Message.Content), usage, nil
}
