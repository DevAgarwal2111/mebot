package tools

import (
	"fmt"
	"log"

	"mebot/types"
)

// ToolHandler is the interface every tool must implement.
type ToolHandler interface {
	Name() string
	Description() string
	Declaration() *types.ToolDeclaration
	Execute(args map[string]any) types.ToolResult
}

// Registry maps tool names to their handlers.
type Registry struct {
	tools map[string]ToolHandler
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]ToolHandler)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t ToolHandler) {
	r.tools[t.Name()] = t
	log.Printf("[Tools] Registered: %s", t.Name())
}

// Execute runs a tool by name with the given args.
func (r *Registry) Execute(name string, args map[string]any) types.ToolResult {
	handler, ok := r.tools[name]
	if !ok {
		return types.ToolResult{
			ToolCallID: name,
			Status:     "not_found",
			Output:     fmt.Sprintf("Tool '%s' not found in registry", name),
		}
	}

	log.Printf("[Tools] Executing: %s with args: %v", name, args)
	result := handler.Execute(args)
	result.ToolCallID = name
	log.Printf("[Tools] Result: %s → %s (output: %d chars)", name, result.Status, len(result.Output))
	return result
}

// GetDeclarations returns all tool declarations for the LLM.
func (r *Registry) GetDeclarations() []*types.ToolDeclaration {
	var decls []*types.ToolDeclaration
	for _, t := range r.tools {
		decls = append(decls, t.Declaration())
	}
	return decls
}

// RegisterDefaults adds all built-in tools.
func (r *Registry) RegisterDefaults() {
	r.Register(&GetCurrentTimeTool{})
	r.Register(&WebRequestTool{})

	// Google API Tools
	r.Register(&GoogleCalendarListEventsTool{})
	r.Register(&GoogleCalendarCreateEventTool{})
	r.Register(&GoogleGmailListUnreadTool{})
	r.Register(&GoogleGmailSendEmailTool{})
}
