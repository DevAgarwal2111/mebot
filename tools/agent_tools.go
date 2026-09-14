package tools

import (
	"context"
	"fmt"
	"log"

	"mebot/config"
	"mebot/llm"
	"mebot/types"
)

// DelegateTaskTool allows the primary agent to spawn a sub-agent on a different provider/model.
type DelegateTaskTool struct {
	Config *config.Config
}

func (t *DelegateTaskTool) Name() string {
	return "delegate_task"
}

func (t *DelegateTaskTool) Description() string {
	return "Delegate a specific task or question to another LLM provider or model."
}

func (t *DelegateTaskTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "delegate_task",
		Description: "Delegate a specific task or question to another LLM provider or model. Use this when a task requires a specific capability (e.g. coding via specialized model) or when the user explicitly requests another model.",
		Parameters: &types.Schema{
			Type: "object",
			Properties: map[string]*types.Schema{
				"provider": {
					Type:        "string",
					Description: "The LLM provider to use: 'gemini', 'nvidia', 'openrouter', or 'ollama'",
				},
				"model": {
					Type:        "string",
					Description: "The exact name of the model to use (e.g., 'anthropic/claude-3.5-sonnet', 'llama3', 'gemini-1.5-pro')",
				},
				"prompt": {
					Type:        "string",
					Description: "The full, highly-detailed prompt and context to pass to the sub-agent. The sub-agent will NOT have access to the conversation history, so you must include ALL necessary context and instructions here.",
				},
			},
			Required: []string{"provider", "model", "prompt"},
		},
	}
}

func (t *DelegateTaskTool) Execute(args map[string]any) types.ToolResult {
	providerName, _ := args["provider"].(string)
	modelName, _ := args["model"].(string)
	prompt, _ := args["prompt"].(string)

	if providerName == "" || modelName == "" || prompt == "" {
		return types.ToolResult{Status: "error", Output: "missing required arguments: provider, model, or prompt"}
	}

	var subProvider llm.Provider
	var err error

	log.Printf("[AgentTools] Delegating task to %s (%s)", providerName, modelName)

	switch providerName {
	case "gemini":
		subProvider, err = llm.NewGeminiProvider(t.Config.GeminiAPIKeys, modelName)
	case "nvidia":
		subProvider, err = llm.NewNvidiaProvider(t.Config.NvidiaAPIKeys, modelName)
	case "openrouter":
		subProvider, err = llm.NewOpenRouterProvider(t.Config.OpenRouterAPIKeys, modelName)
	case "ollama":
		subProvider, err = llm.NewOllamaProvider(t.Config.OllamaEndpoint, modelName)
	default:
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("unknown provider: %s", providerName)}
	}

	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("failed to initialize sub-agent provider (%s): %v", providerName, err)}
	}

	// Build a clean, stateless message history for the sub-agent
	messages := []types.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// We pass empty tools (nil) to the sub-agent to prevent infinite delegation loops
	resp, err := subProvider.SendMessage(context.Background(), messages, nil)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("sub-agent execution failed: %v", err)}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Response from %s (%s):\n\n%s", providerName, modelName, resp.Text),
	}
}
