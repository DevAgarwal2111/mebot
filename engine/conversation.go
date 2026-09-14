package engine

import (
	"mebot/types"
	"sync"
)

// Conversation manages the message history.
type Conversation struct {
	mu           sync.Mutex
	messages     []types.Message
	skillContext string // Injected skill instructions for current turn
}

// NewConversation creates a new conversation with the given system prompt.
func NewConversation(systemPrompt string) *Conversation {
	return &Conversation{
		messages: []types.Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// SetSkillContext sets the skill instructions to inject for the current turn.
func (c *Conversation) SetSkillContext(skillPrompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.skillContext = skillPrompt
}

// AddUserMessage appends a user message.
func (c *Conversation) AddUserMessage(text string, attachments []types.ContentPart) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if len(attachments) == 0 {
		c.messages = append(c.messages, types.Message{Role: "user", Content: text})
	} else {
		var parts []types.ContentPart
		if text != "" {
			parts = append(parts, types.ContentPart{Type: "text", Text: text})
		}
		parts = append(parts, attachments...)
		c.messages = append(c.messages, types.Message{Role: "user", Content: parts})
	}
}

// AddAssistantMessage appends an assistant text response.
func (c *Conversation) AddAssistantMessage(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, types.Message{Role: "assistant", Content: text})
}

// AddToolCalls appends the tool calls the assistant made.
func (c *Conversation) AddToolCalls(toolCalls []types.ToolCall) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, types.Message{Role: "assistant_tool_calls", Content: toolCalls})
}

// AddToolResults appends tool results.
func (c *Conversation) AddToolResults(results []types.ToolResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, types.Message{Role: "tool_result", Content: results})
}

// GetMessages returns a copy of all messages, with skill context injected if present.
func (c *Conversation) GetMessages() []types.Message {
	c.mu.Lock()
	defer c.mu.Unlock()

	msgs := make([]types.Message, len(c.messages))
	copy(msgs, c.messages)

	// If there's active skill context, inject it as a system message after the main system prompt
	if c.skillContext != "" {
		// Insert after the first system message
		skillMsg := types.Message{Role: "system", Content: c.skillContext}
		result := make([]types.Message, 0, len(msgs)+1)
		result = append(result, msgs[0]) // system prompt
		result = append(result, skillMsg) // skill context
		result = append(result, msgs[1:]...) // rest of conversation
		return result
	}

	return msgs
}

// Reset clears the conversation, keeping only the system prompt.
func (c *Conversation) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.messages) > 0 {
		c.messages = c.messages[:1] // keep system prompt
	}
	c.skillContext = ""
}

// IsHistoryTooLong checks if the conversation exceeds the passed limit.
func (c *Conversation) IsHistoryTooLong(limit int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := len(c.messages) - 1
	if count < 0 {
		count = 0
	}
	return count > limit
}

// CompressHistory replaces the old history with a summary message,
// keeping the system prompt and the latest N messages.
func (c *Conversation) CompressHistory(summary string, keepLatest int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.messages) <= 1 {
		return
	}

	systemPrompt := c.messages[0]
	var tail []types.Message

	if len(c.messages)-1 > keepLatest {
		tailStartIndex := len(c.messages) - keepLatest
		tail = make([]types.Message, keepLatest)
		copy(tail, c.messages[tailStartIndex:])
	} else {
		tail = make([]types.Message, len(c.messages)-1)
		copy(tail, c.messages[1:])
	}

	c.messages = []types.Message{
		systemPrompt,
		{Role: "system", Content: "[CONTEXT SUMMARY OF PREVIOUS ACTIONS]:\n" + summary + "\n(End of Summary)."},
	}
	c.messages = append(c.messages, tail...)
}
