package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const anthropicVersion = "2023-06-01"

type AnthropicProvider struct {
	APIKey string
	Model  string
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *AnthropicProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	body := anthropicRequest{
		Model:     p.Model,
		MaxTokens: 256,
		System:    LoadPrompt(),
		Messages: []anthropicMessage{
			{Role: "user", Content: ctx.BuildUserMessage()},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to read response: %w", err)
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", Usage{}, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Error != nil {
		return "", Usage{}, fmt.Errorf("API error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", Usage{}, fmt.Errorf("empty response from API")
	}

	usage := Usage{Model: p.Model}
	if result.Usage != nil {
		usage.InputTokens = result.Usage.InputTokens
		usage.OutputTokens = result.Usage.OutputTokens
	}

	return strings.TrimSpace(result.Content[0].Text), usage, nil
}

func (p *AnthropicProvider) GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error) {
	body := anthropicRequest{
		Model:     p.Model,
		MaxTokens: 256,
		System:    LoadPrompt(),
		Messages: []anthropicMessage{
			{Role: "user", Content: ctx.BuildUserMessage()},
		},
		Stream: true,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", Usage{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", Usage{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", Usage{}, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		var result anthropicResponse
		if json.Unmarshal(respBody, &result) == nil && result.Error != nil {
			return "", Usage{}, fmt.Errorf("API error: %s", result.Error.Message)
		}
		return "", Usage{}, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	var full strings.Builder
	usage := Usage{Model: p.Model}

	parseSSE(resp.Body, func(eventType, data string) {
		switch eventType {
		case "content_block_delta":
			var delta struct {
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &delta) == nil && delta.Delta.Text != "" {
				full.WriteString(delta.Delta.Text)
				onToken(delta.Delta.Text)
			}
		case "message_start":
			var msg struct {
				Message struct {
					Usage *struct {
						InputTokens int `json:"input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if json.Unmarshal([]byte(data), &msg) == nil && msg.Message.Usage != nil {
				usage.InputTokens = msg.Message.Usage.InputTokens
			}
		case "message_delta":
			var msg struct {
				Usage *struct {
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if json.Unmarshal([]byte(data), &msg) == nil && msg.Usage != nil {
				usage.OutputTokens = msg.Usage.OutputTokens
			}
		}
	})

	return strings.TrimSpace(full.String()), usage, nil
}
