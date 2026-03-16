package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"mebot/browser"
	"mebot/types"
)

// ScraperProvider implements llm.Provider by directly typing into an AI chat website.
type ScraperProvider struct {
	ctrl *browser.Controller
}

func NewScraperProvider(ctrl *browser.Controller) (*ScraperProvider, error) {
	if ctrl == nil {
		return nil, fmt.Errorf("BrowserController is required for the scraper provider")
	}
	return &ScraperProvider{ctrl: ctrl}, nil
}

func (p *ScraperProvider) SendMessage(ctx context.Context, messages []types.Message, toolDecls []*types.ToolDeclaration) (*types.LLMResponse, error) {
	// 1. Build the massive string prompt
	var promptBuilder strings.Builder

	promptBuilder.WriteString("YOU ARE A STRICT MACHINE INTERFACE. YOU DO NOT HAVE A CONVERSATIONAL TONE. DO NOT SAY HELLO OR GREET THE USER.\n")
	promptBuilder.WriteString("If you need to execute a tool, you MUST output ONLY a raw JSON block starting EXACTLY with `<TOOL_CALL>` and ending with `</TOOL_CALL>`. Do not write markdown blocks for it.\n\n")

	if len(toolDecls) > 0 {
		promptBuilder.WriteString("AVAILABLE TOOLS:\n")
		declsJSON, _ := json.MarshalIndent(toolDecls, "", "  ")
		promptBuilder.WriteString(string(declsJSON) + "\n\n")

		promptBuilder.WriteString("FORMAT EXPECTED FOR TOOL CALL:\n")
		promptBuilder.WriteString(`<TOOL_CALL>{"id":"call_123", "name":"browser_navigate", "args":{"url":"https://example.com"}}</TOOL_CALL>` + "\n\n")
	}

	promptBuilder.WriteString("CONVERSATION HISTORY:\n")
	for _, msg := range messages {
		switch msg.Role {
		case "system":
			promptBuilder.WriteString(fmt.Sprintf("\n[SYSTEM INSTRUCTION]:\n%v\n", msg.Content))
		case "user":
			promptBuilder.WriteString(fmt.Sprintf("\n[USER]:\n%v\n", msg.Content))
		case "assistant":
			if text, ok := msg.Content.(string); ok && text != "" {
				promptBuilder.WriteString(fmt.Sprintf("\n[ASSISTANT]:\n%v\n", text))
			}
		case "tool_result":
			results, ok := msg.Content.([]types.ToolResult)
			if ok {
				for _, r := range results {
					promptBuilder.WriteString(fmt.Sprintf("\n[TOOL OUTPUT for %s]:\nStatus: %s\n%s\n", r.ToolCallID, r.Status, r.Output))
				}
			}
		}
	}

	promptBuilder.WriteString("\n\nYOUR STRICT INSTRUCTIONS:\n")
	promptBuilder.WriteString("1. If you need information from a URL, use browser_navigate.\n")
	promptBuilder.WriteString("2. If you need to click something, use browser_click.\n")
	promptBuilder.WriteString("3. If you do not need a tool, just answer the user concisely.\n")
	promptBuilder.WriteString("4. NEVER output a conversational greeting. Get straight to the point.\n")
	promptBuilder.WriteString("YOUR NEXT RESPONSE (Output `<TOOL_CALL>{...}</TOOL_CALL>` if you want to use a tool, otherwise just talk):")
	finalPrompt := promptBuilder.String()

	log.Println("[Scraper] Prompt built, ready to inject into browser.")

	// 2. We use our *existing* browser controller to open DuckDuckGo Chat in a new tab
	// Note: We need a way to open a new tab/context without breaking the main navigation tab.
	// For simplicity in this highly experimental PoC, we will hijack the main page.

	// Navigate to DuckDuckGo Chat
	res := p.ctrl.Navigate("https://duckduckgo.com/chat")
	if res.Status == "error" {
		return nil, fmt.Errorf("failed to open DDG Chat: %s", res.Output)
	}

	// This assumes the user manually clicked "Get Started" and agreed to terms previously,
	// or we try to click through the onboarding automatically.
	go func() {
		time.Sleep(2 * time.Second)
		p.ctrl.Click("button:has-text('Get Started')")
		time.Sleep(1 * time.Second)
		p.ctrl.Click("button:has-text('I Agree')")
		time.Sleep(1 * time.Second)
	}()
	time.Sleep(5 * time.Second)

	// Try to find the input box
	// DDG Chat usually has an input type text or textarea
	res = p.ctrl.Type("textarea, input[type='text']", finalPrompt)
	if res.Status == "error" {
		log.Println("[Scraper] Failed to find input box.")
		// Fallback wait and try again
		time.Sleep(3 * time.Second)
		p.ctrl.Type("textarea, input[type='text']", finalPrompt)
	}

	// Press Enter to send
	log.Println("[Scraper] Typing Enter...")
	// We need a specific keyboard press tool, but we can evaluate JS to simulate submission
	// Unfortunately our current controller doesn't expose raw KeyPress. We'll add a quick wrapper in actions.go later.
	// OR we can click the submit button.
	p.ctrl.Click("button[aria-label='Send']") // common aria label

	log.Println("[Scraper] Waiting 15 seconds for generation...")
	time.Sleep(15 * time.Second)

	// Scrape the DOM for the last assistant message
	// DuckDuckGo puts chat bubbles in specific divs. We'll just grab all text and extract the tail.
	ext := p.ctrl.ExtractText()

	return p.parseScrapedText(ext.Output)
}

func (p *ScraperProvider) parseScrapedText(rawText string) (*types.LLMResponse, error) {
	// This is extremely naive and will grab everything.
	// We need to look specifically for our `<TOOL_CALL>` tags anywhere in the body.

	idxStart := strings.Index(rawText, "<TOOL_CALL>")
	idxEnd := strings.Index(rawText, "</TOOL_CALL>")

	result := &types.LLMResponse{Done: true}

	if idxStart != -1 && idxEnd != -1 && idxEnd > idxStart {
		rawJsonStr := rawText[idxStart+11 : idxEnd]

		// Clean the string (AI often escapes quotes or adds markdown inside the tags)
		jsonStr := strings.ReplaceAll(rawJsonStr, "&quot;", "\"")
		jsonStr = strings.ReplaceAll(jsonStr, "\\\"", "\"")
		jsonStr = strings.TrimSpace(jsonStr)
		if strings.HasPrefix(jsonStr, "```json") {
			jsonStr = strings.TrimPrefix(jsonStr, "```json")
			jsonStr = strings.TrimSuffix(jsonStr, "```")
			jsonStr = strings.TrimSpace(jsonStr)
		}

		var tc struct {
			ID   string         `json:"id"`
			Name string         `json:"name"`
			Args map[string]any `json:"args"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &tc); err == nil {
			result.ToolCalls = append(result.ToolCalls, types.ToolCall{
				ID:   tc.ID,
				Name: tc.Name,
				Args: tc.Args,
			})
			result.Done = false

			// Try to get any text the AI said before the tool call
			preText := strings.TrimSpace(rawText[:idxStart])
			// DuckDuckGo includes the whole page text, so just take the last 500 chars before the tool call
			if len(preText) > 500 {
				preText = preText[len(preText)-500:]
			}
			result.Text = preText
		} else {
			log.Printf("[Scraper] Failed to parse JSON block '%s': %v", jsonStr, err)
			result.Text = "Warning: Failed to parse tool execution. Ensure you output valid JSON."
		}
	} else {
		// Just return the raw text (this would normally be filled with garbage from the web page too)
		// Usually you'd extract just the chat bubble container
		// e.g. .message-bubble:last-child
		result.Text = "I read the response from the website. It says... " + rawText[:min(200, len(rawText))]
	}

	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
