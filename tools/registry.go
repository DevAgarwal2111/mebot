package tools

import (
	"fmt"
	"log"
	"strings"

	"mebot/skills"
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

// ToolSummary is a lightweight description of a tool for the system prompt.
type ToolSummary struct {
	Name        string
	Description string
	Category    string
}

// GetToolSummaries returns a summary of all registered tools grouped by category.
func (r *Registry) GetToolSummaries() []ToolSummary {
	var summaries []ToolSummary
	for _, t := range r.tools {
		decl := t.Declaration()
		category := categorize(decl.Name)
		summaries = append(summaries, ToolSummary{
			Name:        decl.Name,
			Description: decl.Description,
			Category:    category,
		})
	}
	return summaries
}

// categorize derives a tool category from its name prefix.
func categorize(name string) string {
	switch {
	case strings.HasPrefix(name, "google_calendar"):
		return "Google Calendar"
	case strings.HasPrefix(name, "google_gmail"):
		return "Gmail"
	case strings.HasPrefix(name, "browser_"):
		return "Browser Automation"
	case strings.HasPrefix(name, "run_command"):
		return "System"
	case strings.HasPrefix(name, "web_"):
		return "Web"
	case strings.Contains(name, "skill"):
		return "Skills & Memory"
	case strings.Contains(name, "reminder"):
		return "Reminders"
	case name == "read_file" || name == "write_file" || name == "list_directory":
		return "File System (Self-Modification)"
	default:
		return "Utility"
	}
}

// RegisterDefaults adds all built-in tools.
func (r *Registry) RegisterDefaults(router *skills.Router, tavilyAPIKey string, broadcast func(types.WSEvent)) {
	r.Register(&GetCurrentTimeTool{})
	r.Register(&WebRequestTool{})

	// Search
	r.Register(&WebSearchTool{APIKey: tavilyAPIKey})

	// Google API Tools
	r.Register(&GoogleCalendarListEventsTool{})
	r.Register(&GoogleCalendarCreateEventTool{})
	r.Register(&GoogleGmailListUnreadTool{})
	r.Register(&GoogleGmailSendEmailTool{})

	// Command Execution
	r.Register(&RunCommandTool{})

	// Async Reminders
	r.Register(&SetReminderTool{Broadcast: broadcast})

	// Skill Management Tools
	r.Register(&CreateSkillTool{Router: router})
	r.Register(&UpdateSkillTool{Router: router})
	r.Register(&ListSkillsTool{Router: router})
	r.Register(&DeleteSkillTool{Router: router})

	// File System Tools (for self-modification and code editing)
	r.Register(&ReadFileTool{})
	r.Register(&WriteFileTool{})
	r.Register(&ListDirectoryTool{})
}
