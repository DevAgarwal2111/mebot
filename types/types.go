package types

// EngineState represents the current state of the engine.
type EngineState string

const (
	StateIdle          EngineState = "idle"
	StateSending       EngineState = "sending"
	StateAwaitingLLM   EngineState = "awaiting_llm"
	StateExecutingTool EngineState = "executing_tool"
	StateAwaitingHuman EngineState = "awaiting_human"
	StateStreaming      EngineState = "streaming"
	StateError         EngineState = "error"
)

// ContentPart represents a chunk of multimodal content (text or file/image).
type ContentPart struct {
	Type     string `json:"type"`                // "text", "image_url", "file"
	Text     string `json:"text,omitempty"`      // For text parts
	MimeType string `json:"mime_type,omitempty"` // e.g. "image/jpeg"
	Data     string `json:"data,omitempty"`      // base64 encoded string
}

// Message represents a conversation message.
type Message struct {
	Role    string      `json:"role"`    // "user", "assistant", "system", "assistant_tool_calls", "tool_result"
	Content interface{} `json:"content"` // string, []ContentPart, []ToolCall, or []ToolResult
}

// ToolCall represents a tool call requested by the LLM.
type ToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
	// Gemini 3.x thought signature fields (opaque, round-tripped as-is)
	Thought          bool   `json:"thought,omitempty"`
	ThoughtSignature []byte `json:"thought_signature,omitempty"`
}

// ToolResult represents the result of executing a tool.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Status     string `json:"status"` // success, error, timeout, not_found
	Output     string `json:"output"`
}

// LLMResponse represents a parsed response from the LLM.
type LLMResponse struct {
	Text      string     `json:"text,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Done      bool       `json:"done"` // true if final answer (no more tool calls)
}

// WSEvent is a WebSocket event sent between engine and frontend.
type WSEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Args    any    `json:"args,omitempty"`
	Status  string `json:"status,omitempty"`
	Step    int    `json:"step,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// ---- Generic Tool Declarations ----

// ToolDeclaration provides a provider-agnostic way to define an LLM function.
type ToolDeclaration struct {
	Name        string
	Description string
	Parameters  *Schema
}

// Schema represents JSON Schema properties for tool arguments.
type Schema struct {
	Type        string
	Description string
	Properties  map[string]*Schema
	Required    []string
}
