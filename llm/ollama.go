package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"mebot/types"

	"github.com/sashabaranov/go-openai"
)

// OllamaProvider wraps the OpenAI SDK configured for local Ollama.
type OllamaProvider struct {
	client *openai.Client
	model  string
}

// NewOllamaProvider initializes an Ollama client via OpenAI compatibility layer.
func NewOllamaProvider(endpoint string, model string) (*OllamaProvider, error) {
	if endpoint == "" {
		endpoint = "http://localhost:11434/v1"
	}

	cfg := openai.DefaultConfig("ollama-dummy-key") // Ollama ignores the key but sdk requires it
	cfg.BaseURL = endpoint

	c := openai.NewClientWithConfig(cfg)

	return &OllamaProvider{
		client: c,
		model:  model,
	}, nil
}

func (p *OllamaProvider) SendMessage(ctx context.Context, messages []types.Message, toolDecls []*types.ToolDeclaration) (*types.LLMResponse, error) {
	var oaiMessages []openai.ChatCompletionMessage

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			text, _ := msg.Content.(string)
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleSystem,
				Content: text,
			})
		case "user":
			text, _ := msg.Content.(string)
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: text,
			})
		case "assistant":
			text, ok := msg.Content.(string)
			if ok && text != "" {
				oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: text,
				})
			}
		case "assistant_tool_calls":
			toolCalls, ok := msg.Content.([]types.ToolCall)
			if !ok {
				continue
			}
			var oaiCalls []openai.ToolCall
			for _, tc := range toolCalls {
				argsJSON, _ := json.Marshal(tc.Args)
				oaiCalls = append(oaiCalls, openai.ToolCall{
					ID:   tc.ID,
					Type: openai.ToolTypeFunction,
					Function: openai.FunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				ToolCalls: oaiCalls,
			})
		case "tool_result":
			results, ok := msg.Content.([]types.ToolResult)
			if !ok {
				continue
			}
			for _, res := range results {
				contentJSON, _ := json.Marshal(map[string]any{
					"status": res.Status,
					"output": res.Output,
				})
				oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    string(contentJSON),
					ToolCallID: res.ToolCallID,
					Name:       res.ToolCallID,
				})
			}
		}
	}

	req := openai.ChatCompletionRequest{
		Model:    p.model,
		Messages: oaiMessages,
	}

	// Add tools
	if len(toolDecls) > 0 {
		for _, decl := range toolDecls {
			req.Tools = append(req.Tools, openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        decl.Name,
					Description: decl.Description,
					Parameters:  translateSchemaToOpenAI(decl.Parameters),
				},
			})
		}
	}

	log.Printf("[LLM] Sending %d messages to Ollama model %s with %d tools", len(oaiMessages), p.model, len(toolDecls))

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("ollama API error: %w", err)
	}

	return p.parseResponse(resp)
}

func (p *OllamaProvider) parseResponse(resp openai.ChatCompletionResponse) (*types.LLMResponse, error) {
	result := &types.LLMResponse{Done: true}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from Ollama")
	}

	msg := resp.Choices[0].Message
	if msg.Content != "" {
		result.Text = msg.Content
	}

	if len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var args map[string]any
			if tc.Function.Arguments != "" {
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					log.Printf("[LLM] warning: failed to parse function arguments: %v", err)
				}
			}
			result.ToolCalls = append(result.ToolCalls, types.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			})
		}
		result.Done = false
	}

	log.Printf("[LLM] Response: text=%d chars, tool_calls=%d, done=%v",
		len(result.Text), len(result.ToolCalls), result.Done)

	return result, nil
}
