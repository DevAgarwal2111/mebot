package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"mebot/types"

	"github.com/sashabaranov/go-openai"
)

// OpenRouterProvider wraps the OpenAI SDK configured for OpenRouter.
type OpenRouterProvider struct {
	clients    []*openai.Client
	model      string
	currentIdx int
	mu         sync.Mutex
}

// NewOpenRouterProvider initializes an OpenRouter client round-robin cluster.
func NewOpenRouterProvider(apiKeys []string, model string) (*OpenRouterProvider, error) {
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("no API keys provided")
	}

	var clients []*openai.Client
	for _, key := range apiKeys {
		if key == "" {
			continue
		}
		cfg := openai.DefaultConfig(key)
		cfg.BaseURL = "https://openrouter.ai/api/v1"

		// Optional: OpenRouter recommended headers
		// (Not fully supported cleanly by default go-openai without custom HTTP client transport, but default works)

		c := openai.NewClientWithConfig(cfg)
		clients = append(clients, c)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid API keys could be initialized")
	}

	return &OpenRouterProvider{
		clients: clients,
		model:   model,
	}, nil
}

func (p *OpenRouterProvider) SendMessage(ctx context.Context, messages []types.Message, toolDecls []*types.ToolDeclaration) (*types.LLMResponse, error) {
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
			if text, ok := msg.Content.(string); ok {
				oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: text,
				})
			} else if parts, ok := msg.Content.([]types.ContentPart); ok {
				var multi []openai.ChatMessagePart
				hasImage := false
				var textBuf string

				for _, p := range parts {
					if p.Type == "text" || p.Type == "" {
						multi = append(multi, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeText,
							Text: p.Data,
						})
						textBuf += p.Data + "\n"
					} else if p.Data != "" {
						hasImage = true
						multi = append(multi, openai.ChatMessagePart{
							Type: openai.ChatMessagePartTypeImageURL,
							ImageURL: &openai.ChatMessageImageURL{
								URL: p.Data,
							},
						})
					}
				}

				if hasImage {
					oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
						Role:         openai.ChatMessageRoleUser,
						MultiContent: multi,
					})
				} else {
					oaiMessages = append(oaiMessages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: textBuf,
					})
				}
			}
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

	p.mu.Lock()
	client := p.clients[p.currentIdx]
	idx := p.currentIdx
	p.currentIdx = (p.currentIdx + 1) % len(p.clients)
	p.mu.Unlock()

	log.Printf("[LLM] Sending %d messages to OpenRouter model %s with %d tools (using key idx %d)", len(oaiMessages), p.model, len(toolDecls), idx)

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("openrouter API error (key %d): %w", idx, err)
	}

	return p.parseResponse(resp)
}

func (p *OpenRouterProvider) parseResponse(resp openai.ChatCompletionResponse) (*types.LLMResponse, error) {
	result := &types.LLMResponse{Done: true}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenRouter")
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

// translateSchemaToOpenAI converts our generic Schema to openrouter/openai JSON schema.
func translateSchemaToOpenAI(s *types.Schema) any {
	if s == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}

	schema := map[string]any{
		"type":        s.Type,
		"description": s.Description,
	}

	if len(s.Properties) > 0 {
		props := make(map[string]any)
		for k, v := range s.Properties {
			props[k] = translateSchemaToOpenAI(v)
		}
		schema["properties"] = props
	}

	if len(s.Required) > 0 {
		schema["required"] = s.Required
	}

	return schema
}
