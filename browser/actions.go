package browser

import (
	"fmt"
	"mebot/types"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// Navigate goes to a URL and waits for the network to idle.
func (c *Controller) Navigate(url string) types.ToolResult {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	_, err := c.page.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(15000), // 15 sec timeout
	})

	// Wait a tiny bit extra for JS to render
	time.Sleep(1 * time.Second)

	status := "success"
	output := fmt.Sprintf("Navigated to %s", c.page.URL())
	if err != nil {
		status = "error"
		output = fmt.Sprintf("Navigation to %s failed: %v", url, err)
	}

	return types.ToolResult{
		Status: status,
		Output: output,
	}
}

// Click finds an element by selector and clicks it.
func (c *Controller) Click(selector string) types.ToolResult {
	err := c.page.Click(selector, playwright.PageClickOptions{
		Timeout: playwright.Float(15000),
	})

	// Wait a moment for navigation or DOM updates after clicking
	time.Sleep(1 * time.Second)

	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to click '%s': %v", selector, err),
		}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Clicked element matching '%s'", selector),
	}
}

// Type finds an input element by selector and types text into it.
func (c *Controller) Type(selector string, text string) types.ToolResult {
	err := c.page.Fill(selector, text, playwright.PageFillOptions{
		Timeout: playwright.Float(15000),
	})

	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to type into '%s': %v", selector, err),
		}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Typed text into '%s'", selector),
	}
}

// PressKey presses a keyboard key (e.g. "Enter", "Tab", "Escape").
func (c *Controller) PressKey(key string) types.ToolResult {
	err := c.page.Keyboard().Press(key)
	time.Sleep(1 * time.Second) // wait for result of keystroke

	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to press key '%s': %v", key, err),
		}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Pressed key '%s'", key),
	}
}

// Scroll scrolls the page up or down by the given pixel amount.
func (c *Controller) Scroll(direction string, amount int) types.ToolResult {
	delta := amount
	if direction == "up" {
		delta = -amount
	}

	_, err := c.page.Evaluate(fmt.Sprintf("window.scrollBy(0, %d)", delta))
	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to scroll: %v", err),
		}
	}

	time.Sleep(500 * time.Millisecond)

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Scrolled %s by %d pixels", direction, amount),
	}
}

// WaitForSelector waits for an element matching the selector to appear on the page.
func (c *Controller) WaitForSelector(selector string, timeoutMs float64) types.ToolResult {
	if timeoutMs <= 0 {
		timeoutMs = 10000 // default 10 seconds
	}

	_, err := c.page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(timeoutMs),
	})

	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Element '%s' did not appear within %.0fms: %v", selector, timeoutMs, err),
		}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Element '%s' is now visible on the page", selector),
	}
}

// EvalJS executes arbitrary JavaScript on the current page and returns the result.
func (c *Controller) EvalJS(script string) types.ToolResult {
	result, err := c.page.Evaluate(script)
	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("JavaScript execution failed: %v", err),
		}
	}

	output := fmt.Sprintf("%v", result)
	if len(output) > 5000 {
		output = output[:5000] + "\n... (truncated)"
	}

	return types.ToolResult{
		Status: "success",
		Output: output,
	}
}

// SelectOption selects a dropdown option by its value.
func (c *Controller) SelectOption(selector string, value string) types.ToolResult {
	_, err := c.page.SelectOption(selector, playwright.SelectOptionValues{
		Values: playwright.StringSlice(value),
	}, playwright.PageSelectOptionOptions{
		Timeout: playwright.Float(10000),
	})

	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Failed to select option '%s' in '%s': %v", value, selector, err),
		}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Selected option '%s' in dropdown '%s'", value, selector),
	}
}
