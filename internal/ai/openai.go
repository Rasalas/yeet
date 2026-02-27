package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type OpenAIProvider struct {
	APIKey  string
	Model   string
	BaseURL string
}

type openaiRequest struct {
	Model         string            `json:"model"`
	Messages      []openaiMessage   `json:"messages"`
	Stream        bool              `json:"stream,omitempty"`
	StreamOptions *openaiStreamOpts `json:"stream_options,omitempty"`
}

type openaiStreamOpts struct {
	IncludeUsage bool `json:"include_usage"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *OpenAIProvider) baseURL() string {
	if p.BaseURL != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return "https://api.openai.com/v1"
}

func (p *OpenAIProvider) headers() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + p.APIKey,
	}
}

func (p *OpenAIProvider) GenerateCommitMessage(ctx CommitContext) (string, Usage, error) {
	body := openaiRequest{
		Model: p.Model,
		Messages: []openaiMessage{
			{Role: "system", Content: ctx.EffectivePrompt()},
			{Role: "user", Content: ctx.BuildUserMessage()},
		},
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	var result openaiResponse
	if err := doRequest(reqCtx, "POST", p.baseURL()+"/chat/completions", body, p.headers(), &result); err != nil {
		return "", Usage{}, err
	}

	if result.Error != nil {
		return "", Usage{}, fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("empty response from API")
	}

	usage := Usage{Model: p.Model}
	if result.Usage != nil {
		usage.InputTokens = result.Usage.PromptTokens
		usage.OutputTokens = result.Usage.CompletionTokens
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), usage, nil
}

func (p *OpenAIProvider) GenerateCommitMessageStream(ctx CommitContext, onToken func(string)) (string, Usage, error) {
	body := openaiRequest{
		Model: p.Model,
		Messages: []openaiMessage{
			{Role: "system", Content: ctx.EffectivePrompt()},
			{Role: "user", Content: ctx.BuildUserMessage()},
		},
		Stream:        true,
		StreamOptions: &openaiStreamOpts{IncludeUsage: true},
	}

	resp, err := doStream(context.Background(), p.baseURL()+"/chat/completions", body, p.headers())
	if err != nil {
		return "", Usage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		var result openaiResponse
		if json.Unmarshal(respBody, &result) == nil && result.Error != nil {
			return "", Usage{}, fmt.Errorf("API error: %s", result.Error.Message)
		}
		return "", Usage{}, fmt.Errorf("API error: status %d", resp.StatusCode)
	}

	var full strings.Builder
	usage := Usage{Model: p.Model}

	if err := parseSSE(resp.Body, func(eventType, data string) {
		if data == "[DONE]" {
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}

		if json.Unmarshal([]byte(data), &chunk) != nil {
			return
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			token := chunk.Choices[0].Delta.Content
			full.WriteString(token)
			onToken(token)
		}

		if chunk.Usage != nil {
			usage.InputTokens = chunk.Usage.PromptTokens
			usage.OutputTokens = chunk.Usage.CompletionTokens
		}
	}); err != nil {
		return "", Usage{}, fmt.Errorf("stream read error: %w", err)
	}

	return strings.TrimSpace(full.String()), usage, nil
}
