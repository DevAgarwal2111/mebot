package browser

import (
	"encoding/base64"
	"fmt"
	"mebot/types"

	"github.com/playwright-community/playwright-go"
)

// Screenshot explicitly takes a screenshot and returns it.
func (c *Controller) Screenshot() types.ToolResult {
	title, _ := c.page.Title()

	// Truncate title if very long
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	options := playwright.PageScreenshotOptions{
		Type:    playwright.ScreenshotTypeJpeg,
		Quality: playwright.Int(50),
	}

	screenshotBytes, err := c.page.Screenshot(options)
	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to take screenshot: %v", err),
		}
	}

	base64Shot := base64.StdEncoding.EncodeToString(screenshotBytes)
	_ = base64Shot // We encode it for the frontend but don't send it to the LLM anymore

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Screenshot taken. Current page: %s (%s)", title, c.page.URL()),
	}
}

// ExtractText gets all the visible text on the page for LLM reading.
func (c *Controller) ExtractText() types.ToolResult {
	text, err := c.page.Evaluate(`() => document.body.innerText`)
	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to extract text: %v", err),
		}
	}

	textStr, _ := text.(string)
	if len(textStr) > 5000 {
		textStr = textStr[:5000] + "\n... (truncated)"
	}

	return types.ToolResult{
		Status: "success",
		Output: textStr,
	}
}

// ExtractDOM returns a summarized version of the DOM (inputs, buttons, links) to help the LLM find selectors.
func (c *Controller) ExtractDOM() types.ToolResult {
	script := `() => {
		const elements = document.querySelectorAll('input, button, a, select, textarea');
		return Array.from(elements).map(e => {
			let tag = e.tagName.toLowerCase();
			let id = e.id ? ' id="' + e.id + '"' : '';
			let name = e.name ? ' name="' + e.name + '"' : '';
			let type = e.type ? ' type="' + e.type + '"' : '';
			let placeholder = e.placeholder ? ' placeholder="' + e.placeholder + '"' : '';
			let text = (e.innerText || e.value || '').trim().substring(0, 50);
			return '<' + tag + id + name + type + placeholder + '>' + text + '</' + tag + '>';
		}).join('\n');
	}`

	result, err := c.page.Evaluate(script)
	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to extract DOM elements: %v", err),
		}
	}

	resStr, _ := result.(string)
	if len(resStr) > 8000 {
		resStr = resStr[:8000] + "\n... (truncated)"
	}

	if resStr == "" {
		resStr = "No interactive elements found."
	}

	return types.ToolResult{
		Status: "success",
		Output: "Interactive Elements Available:\n\n" + resStr,
	}
}
