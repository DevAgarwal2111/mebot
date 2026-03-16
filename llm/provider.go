package llm

import (
	"context"

	"mebot/types"
)

// Provider is the common interface for LLM backends (Gemini, OpenRouter, etc).
type Provider interface {
	SendMessage(ctx context.Context, messages []types.Message, tools []*types.ToolDeclaration) (*types.LLMResponse, error)
}
