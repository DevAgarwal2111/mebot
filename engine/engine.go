package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	googleauth "mebot/api/google"
	"mebot/config"
	"mebot/llm"
	"mebot/skills"
	"mebot/tools"
	"mebot/types"
)

// Engine is the core agentic loop that coordinates LLM calls and tool execution.
type Engine struct {
	mu           sync.Mutex
	state        types.EngineState
	cfg          *config.Config
	llmClient    llm.Provider
	toolRegistry *tools.Registry
	conversation *Conversation
	skillRouter  *skills.Router
}

// NewEngine creates a new engine instance with a dynamically built system prompt.
func NewEngine(cfg *config.Config, llmClient llm.Provider, toolRegistry *tools.Registry, skillRouter *skills.Router) *Engine {
	prompt := buildDynamicPrompt(toolRegistry, skillRouter)

	return &Engine{
		state:        types.StateIdle,
		cfg:          cfg,
		llmClient:    llmClient,
		toolRegistry: toolRegistry,
		conversation: NewConversation(prompt),
		skillRouter:  skillRouter,
	}
}

// buildDynamicPrompt assembles the system prompt from current runtime state.
func buildDynamicPrompt(toolRegistry *tools.Registry, skillRouter *skills.Router) string {
	toolSummaries := toolRegistry.GetToolSummaries()

	var allSkills []*skills.Skill
	if skillRouter != nil {
		allSkills = skillRouter.GetLoader().GetAll()
	}

	integrations := map[string]string{
		"Google (Gmail & Calendar)": googleauth.GetStatus(),
	}

	return BuildSystemPrompt(toolSummaries, allSkills, integrations)
}

// HandleMessage processes a user message through the agentic loop.
// It sends events via the provided callback as the loop progresses.
func (e *Engine) HandleMessage(ctx context.Context, userMsg string, sendEvent func(types.WSEvent)) {
	e.mu.Lock()
	if e.state != types.StateIdle {
		e.mu.Unlock()
		sendEvent(types.WSEvent{Type: "error", Content: "Engine is busy processing another request"})
		return
	}
	e.state = types.StateSending
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.state = types.StateIdle
		e.mu.Unlock()
	}()

	// Add user message to conversation
	e.conversation.AddUserMessage(userMsg)

	// Send "thinking" to frontend
	sendEvent(types.WSEvent{Type: "thinking", Content: "Processing your request..."})

	// === SKILL ROUTING ===
	// Match relevant skills for this message and inject into conversation context
	if e.skillRouter != nil {
		matchedSkills := e.skillRouter.Route(userMsg)

		// Check if we have a user identity — if not, inject onboarding instructions
		_, hasIdentity := e.skillRouter.GetLoader().Get("user_identity")
		var extraContext string
		if !hasIdentity {
			extraContext = `
[ONBOARDING - FIRST MEETING]
You have NOT met this user before. This is your first interaction.
1. Introduce yourself warmly — ask the user what they'd like to call you (your bot name)
2. Ask for their name
3. Ask about any preferences (timezone, communication style, etc.)
4. Once you have this info, IMMEDIATELY save it using the 'create_skill' tool with:
   - name: "user_identity"
   - description: "Core identity and user information"
   - triggers: (leave empty — this skill is loaded for context, not triggered)
   - content: Include the bot's chosen name, the user's name, and any preferences they mention
This is CRITICAL — do not skip this step. The user_identity skill is your persistent memory.
`
		}

		skillPrompt := e.skillRouter.FormatForPrompt(matchedSkills)
		if extraContext != "" {
			skillPrompt = extraContext + skillPrompt
		}

		if skillPrompt != "" {
			e.conversation.SetSkillContext(skillPrompt)
			log.Printf("[Engine] Injected %d skills for this turn (identity: %v)", len(matchedSkills), hasIdentity)
		} else {
			e.conversation.SetSkillContext("")
		}
	}

	// Create a timeout context for the entire loop
	loopCtx, cancel := context.WithTimeout(ctx, time.Duration(e.cfg.ToolTimeoutSecs*e.cfg.MaxToolIterations)*time.Second)
	defer cancel()

	// Get tool declarations
	toolDecls := e.toolRegistry.GetDeclarations()

	// === INNER AGENTIC LOOP ===
	for iteration := 0; iteration < e.cfg.MaxToolIterations; iteration++ {
		// Check context
		if loopCtx.Err() != nil {
			sendEvent(types.WSEvent{Type: "error", Content: "Request timed out"})
			return
		}

		// Send conversation to LLM
		e.setState(types.StateAwaitingLLM)
		sendEvent(types.WSEvent{
			Type:    "task_progress",
			Content: fmt.Sprintf("Thinking... (step %d)", iteration+1),
			Step:    iteration + 1,
			Total:   e.cfg.MaxToolIterations,
		})

		messages := e.conversation.GetMessages()
		resp, err := e.llmClient.SendMessage(loopCtx, messages, toolDecls)

		// Log the conversation data here
		logMsgs := ""
		for _, m := range messages {
			logMsgs += fmt.Sprintf("[%s]: %v\n", m.Role, m.Content)
		}

		if err == nil {
			logMsgs += fmt.Sprintf("\n[LLM Response (Step %d)]:\nText: %s\nToolCalls: %v\nDone: %v\n", iteration+1, resp.Text, resp.ToolCalls, resp.Done)
		} else {
			logMsgs += fmt.Sprintf("\n[LLM Error (Step %d)]: %v\n", iteration+1, err)
		}

		f, logErr := os.OpenFile("llm_conversation.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if logErr == nil {
			f.WriteString(fmt.Sprintf("\n\n=== ITERATION %d ===\n%s\n", iteration+1, logMsgs))
			f.Close()
		} else {
			log.Printf("[Engine] Failed to write to llm_conversation.log: %v", logErr)
		}

		if err != nil {
			log.Printf("[Engine] LLM error: %v", err)
			sendEvent(types.WSEvent{Type: "error", Content: fmt.Sprintf("LLM error: %v", err)})
			return
		}

		// If we have a final text answer and no tool calls → done
		if resp.Done {
			e.setState(types.StateStreaming)
			e.conversation.AddAssistantMessage(resp.Text)
			sendEvent(types.WSEvent{Type: "assistant", Content: resp.Text})
			sendEvent(types.WSEvent{Type: "task_complete", Content: "Done"})
			return
		}

		// We have tool calls — execute them
		if len(resp.ToolCalls) > 0 {
			// If there's also text alongside tool calls, send it
			if resp.Text != "" {
				sendEvent(types.WSEvent{Type: "thinking", Content: resp.Text})
			}

			// Record tool calls in conversation
			e.conversation.AddToolCalls(resp.ToolCalls)

			// Execute each tool SEQUENTIALLY
			// In a Multi-Step plan, if one tool fails, we must ABORT the remainder of the queue.
			e.setState(types.StateExecutingTool)
			var results []types.ToolResult

			for i, tc := range resp.ToolCalls {
				// Notify frontend about the tool call
				sendEvent(types.WSEvent{
					Type: "tool_call",
					Tool: tc.Name,
					Args: tc.Args,
				})

				// Execute the tool
				result := e.toolRegistry.Execute(tc.Name, tc.Args)
				result.ToolCallID = tc.Name
				results = append(results, result)

				// Notify frontend about the result
				sendEvent(types.WSEvent{
					Type:    "tool_result",
					Tool:    tc.Name,
					Status:  result.Status,
					Content: result.Output,
				})

				// EARLY ABORT: If a tool errored, the subsequent planned tools will likely
				// be hallucinating off a broken state. Cancel them and return early to LLM.
				if result.Status == "error" && i < len(resp.ToolCalls)-1 {
					sendEvent(types.WSEvent{
						Type:    "thinking",
						Content: fmt.Sprintf("Tool '%s' failed. Aborting remaining %d planned steps.", tc.Name, len(resp.ToolCalls)-(i+1)),
					})
					log.Printf("[Engine] Tool '%s' failed. Aborting remaining %d planned steps.", tc.Name, len(resp.ToolCalls)-(i+1))
					break
				}
			}

			// Add results to conversation and loop back
			e.conversation.AddToolResults(results)
			continue
		}

		// Edge case: no text and no tool calls
		sendEvent(types.WSEvent{Type: "assistant", Content: "I processed your request but have nothing to report."})
		return
	}

	// Max iterations exceeded
	sendEvent(types.WSEvent{
		Type:    "error",
		Content: fmt.Sprintf("Reached maximum of %d iterations without a final answer", e.cfg.MaxToolIterations),
	})
}

// setState updates the engine state thread-safely.
func (e *Engine) setState(state types.EngineState) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state = state
	log.Printf("[Engine] State → %s", state)
}

// ResetConversation clears the conversation history and rebuilds the system prompt.
func (e *Engine) ResetConversation() {
	prompt := buildDynamicPrompt(e.toolRegistry, e.skillRouter)
	e.conversation = NewConversation(prompt)
	log.Println("[Engine] Conversation reset with fresh dynamic prompt")
}
