package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"

	"google.golang.org/genai"

	"mebot/types"
)

// GeminiProvider wraps the Gemini API for chat with function calling.
type GeminiProvider struct {
	clients    []*genai.Client
	model      string
	currentIdx int
	mu         sync.Mutex
}

// NewGeminiProvider creates a new Gemini LLM client instance for each API key provided.
func NewGeminiProvider(apiKeys []string, model string) (*GeminiProvider, error) {
	if len(apiKeys) == 0 {
		return nil, fmt.Errorf("no API keys provided")
	}

	var clients []*genai.Client
	for _, key := range apiKeys {
		if key == "" {
			continue
		}
		c, err := genai.NewClient(context.Background(), &genai.ClientConfig{
			APIKey:  key,
			Backend: genai.BackendGeminiAPI,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create genai client for a key: %w", err)
		}
		clients = append(clients, c)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid API keys could be initialized")
	}

	return &GeminiProvider{
		clients: clients,
		model:   model,
	}, nil
}

// SendMessage sends the conversation history + tool declarations to Gemini
// and returns the parsed response.
func (c *GeminiProvider) SendMessage(ctx context.Context, messages []types.Message, toolDecls []*types.ToolDeclaration) (*types.LLMResponse, error) {
	// Build the contents array from conversation history
	var contents []*genai.Content
	var systemInstruction *genai.Content

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			text, _ := msg.Content.(string)
			systemInstruction = &genai.Content{
				Parts: []*genai.Part{genai.NewPartFromText(text)},
				Role:  "user", // Gemini uses system instruction differently
			}
		case "user":
			if text, ok := msg.Content.(string); ok {
				contents = append(contents, &genai.Content{
					Parts: []*genai.Part{genai.NewPartFromText(text)},
					Role:  "user",
				})
			} else if parts, ok := msg.Content.([]types.ContentPart); ok {
				var genaiParts []*genai.Part
				for _, p := range parts {
					if p.Type == "text" {
						genaiParts = append(genaiParts, genai.NewPartFromText(p.Text))
					} else if p.Data != "" {
						dataStr := p.Data
						// Remove 'data:image/jpeg;base64,' prefix if present
						if idx := strings.Index(dataStr, ","); idx != -1 {
							dataStr = dataStr[idx+1:]
						}
						decoded, err := base64.StdEncoding.DecodeString(dataStr)
						if err == nil {
							mime := p.MimeType
							if mime == "" {
								mime = "image/jpeg"
							}
							genaiParts = append(genaiParts, &genai.Part{
								InlineData: &genai.Blob{
									MIMEType: mime,
									Data:     decoded,
								},
							})
						} else {
							log.Printf("[LLM] Failed to decode attachment base64: %v", err)
						}
					}
				}
				contents = append(contents, &genai.Content{
					Parts: genaiParts,
					Role:  "user",
				})
			}
		case "assistant":
			text, ok := msg.Content.(string)
			if ok && text != "" {
				contents = append(contents, &genai.Content{
					Parts: []*genai.Part{genai.NewPartFromText(text)},
					Role:  "model",
				})
			}
			// If the content is tool calls, we already handled them
		case "assistant_tool_calls":
			// Reconstruct the function call parts
			toolCalls, ok := msg.Content.([]types.ToolCall)
			if !ok {
				continue
			}
			var parts []*genai.Part
			for _, tc := range toolCalls {
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						Name: tc.Name,
						Args: tc.Args,
					},
					Thought:          tc.Thought,
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
			contents = append(contents, &genai.Content{
				Parts: parts,
				Role:  "model",
			})
		case "tool_result":
			// Tool results
			results, ok := msg.Content.([]types.ToolResult)
			if !ok {
				continue
			}
			var parts []*genai.Part
			for _, tr := range results {
				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name: tr.ToolCallID,
						Response: map[string]any{
							"status": tr.Status,
							"output": tr.Output,
						},
					},
				})
			}
			contents = append(contents, &genai.Content{
				Parts: parts,
				Role:  "user",
			})
		}
	}

	// Build config
	config := &genai.GenerateContentConfig{}
	if systemInstruction != nil {
		config.SystemInstruction = systemInstruction
	}

	// Add tools if we have function declarations
	if len(toolDecls) > 0 {
		var genaiTools []*genai.FunctionDeclaration
		for _, decl := range toolDecls {
			genaiTools = append(genaiTools, translateTool(decl))
		}
		config.Tools = []*genai.Tool{
			{FunctionDeclarations: genaiTools},
		}
	}

	// Round-robin key selection
	c.mu.Lock()
	client := c.clients[c.currentIdx]
	idx := c.currentIdx
	c.currentIdx = (c.currentIdx + 1) % len(c.clients)
	c.mu.Unlock()

	// Call the API
	log.Printf("[LLM] Sending %d messages to %s with %d tools (using key %d)", len(contents), c.model, len(toolDecls), idx)
	resp, err := client.Models.GenerateContent(ctx, c.model, contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini API error (key %d): %w", idx, err)
	}

	// Parse the response
	return c.parseResponse(resp)
}

// parseResponse converts a Gemini response into our internal LLMResponse.
func (c *GeminiProvider) parseResponse(resp *genai.GenerateContentResponse) (*types.LLMResponse, error) {
	result := &types.LLMResponse{Done: true}

	if resp == nil || len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return result, nil
	}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			result.Text += part.Text
		}
		if part.FunctionCall != nil {
			result.ToolCalls = append(result.ToolCalls, types.ToolCall{
				ID:               part.FunctionCall.Name, // Gemini doesn't use separate IDs
				Name:             part.FunctionCall.Name,
				Args:             part.FunctionCall.Args,
				Thought:          part.Thought,
				ThoughtSignature: part.ThoughtSignature,
			})
			result.Done = false
		}
	}

	log.Printf("[LLM] Response: text=%d chars, tool_calls=%d, done=%v",
		len(result.Text), len(result.ToolCalls), result.Done)

	return result, nil
}

// translateTool converts our generic ToolDeclaration into Gemini's format.
func translateTool(decl *types.ToolDeclaration) *genai.FunctionDeclaration {
	return &genai.FunctionDeclaration{
		Name:        decl.Name,
		Description: decl.Description,
		Parameters:  translateSchema(decl.Parameters),
	}
}

// translateSchema recursively converts our generic Schema into Gemini's format.
func translateSchema(s *types.Schema) *genai.Schema {
	if s == nil {
		return nil
	}

	props := make(map[string]*genai.Schema)
	for k, v := range s.Properties {
		props[k] = translateSchema(v)
	}

	return &genai.Schema{
		Type:        genai.Type(s.Type),
		Description: s.Description,
		Properties:  props,
		Required:    s.Required,
	}
}
