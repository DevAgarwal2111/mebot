package tools

import (
	"mebot/browser"

	"mebot/types"
)

// ---- browser_navigate ----

type BrowserNavigateTool struct {
	ctrl *browser.Controller
}

func (t *BrowserNavigateTool) Name() string        { return "browser_navigate" }
func (t *BrowserNavigateTool) Description() string { return "Navigate the browser to a given URL" }

func (t *BrowserNavigateTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_navigate",
		Description: "Navigate the browser to a given URL. Always returns a screenshot of the resulting page state.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"url": {
					Type:        "STRING",
					Description: "The absolute URL to navigate to (e.g., 'https://wikipedia.org')",
				},
			},
			Required: []string{"url"},
		},
	}
}

func (t *BrowserNavigateTool) Execute(args map[string]any) types.ToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: url"}
	}

	// Execute action
	res := t.ctrl.Navigate(url)

	// Automatically append the screenshot after the action
	if res.Status != "error" {
		shot := t.ctrl.Screenshot()
		res.Output += "\n\n" + shot.Output
		// In a real implementation we would attach the actual base64 image data to the result object
		// res.ScreenshotBase64 = t.ctrl.GetScreenshotBase64()
	}
	return res
}

// ---- browser_click ----

type BrowserClickTool struct {
	ctrl *browser.Controller
}

func (t *BrowserClickTool) Name() string        { return "browser_click" }
func (t *BrowserClickTool) Description() string { return "Click on an element in the browser" }

func (t *BrowserClickTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_click",
		Description: "Click on an element matching a given CSS selector.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"selector": {
					Type:        "STRING",
					Description: "A valid CSS selector (e.g. 'button#submit' or '.link-item')",
				},
			},
			Required: []string{"selector"},
		},
	}
}

func (t *BrowserClickTool) Execute(args map[string]any) types.ToolResult {
	sel, ok := args["selector"].(string)
	if !ok || sel == "" {
		return types.ToolResult{Status: "error", Output: "Missing 'selector' argument"}
	}
	res := t.ctrl.Click(sel)
	if res.Status != "error" {
		res.Output += "\n\n" + t.ctrl.Screenshot().Output
	}
	return res
}

// ---- browser_type ----

type BrowserTypeTool struct {
	ctrl *browser.Controller
}

func (t *BrowserTypeTool) Name() string        { return "browser_type" }
func (t *BrowserTypeTool) Description() string { return "Type text into an input field" }

func (t *BrowserTypeTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_type",
		Description: "Fill a text input field with the provided text.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"selector": {
					Type:        "STRING",
					Description: "CSS selector for the input element",
				},
				"text": {
					Type:        "STRING",
					Description: "The text to type into the field",
				},
			},
			Required: []string{"selector", "text"},
		},
	}
}

func (t *BrowserTypeTool) Execute(args map[string]any) types.ToolResult {
	sel, _ := args["selector"].(string)
	text, _ := args["text"].(string)
	res := t.ctrl.Type(sel, text)
	if res.Status != "error" {
		res.Output += "\n\n" + t.ctrl.Screenshot().Output
	}
	return res
}

// ---- browser_extract_dom ----

type BrowserExtractDOMTool struct {
	ctrl *browser.Controller
}

func (t *BrowserExtractDOMTool) Name() string { return "browser_extract_dom" }
func (t *BrowserExtractDOMTool) Description() string {
	return "Extract interactive elements from the page to find selectors"
}

func (t *BrowserExtractDOMTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_extract_dom",
		Description: "Extracts an HTML summary of all inputs, buttons, and links on the current page to help identify correct selectors for clicking or typing.",
		Parameters: &types.Schema{
			Type:       "OBJECT",
			Properties: map[string]*types.Schema{},
		},
	}
}

func (t *BrowserExtractDOMTool) Execute(args map[string]any) types.ToolResult {
	return t.ctrl.ExtractDOM()
}

// ---- browser_scroll ----

type BrowserScrollTool struct {
	ctrl *browser.Controller
}

func (t *BrowserScrollTool) Name() string        { return "browser_scroll" }
func (t *BrowserScrollTool) Description() string { return "Scroll the page up or down" }

func (t *BrowserScrollTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_scroll",
		Description: "Scroll the page up or down by a specific pixel amount. Use this to see content that is off-screen.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"direction": {
					Type:        "STRING",
					Description: "Direction to scroll ('up' or 'down')",
				},
				"amount": {
					Type:        "INTEGER",
					Description: "Amount of pixels to scroll (e.g. 500)",
				},
			},
			Required: []string{"direction", "amount"},
		},
	}
}

func (t *BrowserScrollTool) Execute(args map[string]any) types.ToolResult {
	direction, _ := args["direction"].(string)

	// Handle float64 (from JSON decoding) or int
	amount := 500
	if act, ok := args["amount"].(float64); ok {
		amount = int(act)
	} else if act, ok := args["amount"].(int); ok {
		amount = act
	}

	if direction != "up" && direction != "down" {
		return types.ToolResult{Status: "error", Output: "direction must be 'up' or 'down'"}
	}

	res := t.ctrl.Scroll(direction, amount)
	if res.Status != "error" {
		res.Output += "\n\n" + t.ctrl.Screenshot().Output
	}
	return res
}

// ---- browser_wait_for_selector ----

type BrowserWaitForSelectorTool struct {
	ctrl *browser.Controller
}

func (t *BrowserWaitForSelectorTool) Name() string { return "browser_wait_for_selector" }
func (t *BrowserWaitForSelectorTool) Description() string {
	return "Wait for an element to appear on the page"
}

func (t *BrowserWaitForSelectorTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_wait_for_selector",
		Description: "Wait for a specific CSS selector to appear in the DOM. Use this when a page is loading dynamic content.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"selector": {
					Type:        "STRING",
					Description: "CSS selector to wait for",
				},
				"timeout_ms": {
					Type:        "INTEGER",
					Description: "Timeout in milliseconds (e.g. 10000 for 10 seconds)",
				},
			},
			Required: []string{"selector"},
		},
	}
}

func (t *BrowserWaitForSelectorTool) Execute(args map[string]any) types.ToolResult {
	sel, ok := args["selector"].(string)
	if !ok || sel == "" {
		return types.ToolResult{Status: "error", Output: "Missing 'selector' argument"}
	}

	timeoutMs := float64(10000)
	if tOut, ok := args["timeout_ms"].(float64); ok {
		timeoutMs = tOut
	}

	res := t.ctrl.WaitForSelector(sel, timeoutMs)
	if res.Status != "error" {
		res.Output += "\n\n" + t.ctrl.Screenshot().Output
	}
	return res
}

// ---- browser_eval_js ----

type BrowserEvalJSTool struct {
	ctrl *browser.Controller
}

func (t *BrowserEvalJSTool) Name() string        { return "browser_eval_js" }
func (t *BrowserEvalJSTool) Description() string { return "Evaluate arbitrary JavaScript on the page" }

func (t *BrowserEvalJSTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_eval_js",
		Description: "Run arbitrary JavaScript on the current page and return the result. Useful for complex DOM querying, removing annoying popups/overlays by ID, or grabbing data.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"script": {
					Type:        "STRING",
					Description: "The JavaScript code to execute.",
				},
			},
			Required: []string{"script"},
		},
	}
}

func (t *BrowserEvalJSTool) Execute(args map[string]any) types.ToolResult {
	script, ok := args["script"].(string)
	if !ok || script == "" {
		return types.ToolResult{Status: "error", Output: "Missing 'script' argument"}
	}

	return t.ctrl.EvalJS(script)
}

// ---- browser_select_option ----

type BrowserSelectOptionTool struct {
	ctrl *browser.Controller
}

func (t *BrowserSelectOptionTool) Name() string { return "browser_select_option" }
func (t *BrowserSelectOptionTool) Description() string {
	return "Select an option in a dropdown element"
}

func (t *BrowserSelectOptionTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_select_option",
		Description: "Select an option from a <select> dropdown by its value.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"selector": {
					Type:        "STRING",
					Description: "CSS selector for the <select> element",
				},
				"value": {
					Type:        "STRING",
					Description: "The value of the option to select",
				},
			},
			Required: []string{"selector", "value"},
		},
	}
}

func (t *BrowserSelectOptionTool) Execute(args map[string]any) types.ToolResult {
	sel, okSel := args["selector"].(string)
	val, okVal := args["value"].(string)
	if !okSel || !okVal || sel == "" || val == "" {
		return types.ToolResult{Status: "error", Output: "Missing 'selector' or 'value' argument"}
	}

	res := t.ctrl.SelectOption(sel, val)
	if res.Status != "error" {
		res.Output += "\n\n" + t.ctrl.Screenshot().Output
	}
	return res
}

// ---- browser_press_key ----

type BrowserPressKeyTool struct {
	ctrl *browser.Controller
}

func (t *BrowserPressKeyTool) Name() string        { return "browser_press_key" }
func (t *BrowserPressKeyTool) Description() string { return "Press a keyboard key" }

func (t *BrowserPressKeyTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "browser_press_key",
		Description: "Press a keyboard key. Extremely useful for pressing 'Enter' to submit forms or 'Escape' to dismiss modals.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"key": {
					Type:        "STRING",
					Description: "Name of the key to press (e.g. 'Enter', 'Escape', 'Tab', 'ArrowDown')",
				},
			},
			Required: []string{"key"},
		},
	}
}

func (t *BrowserPressKeyTool) Execute(args map[string]any) types.ToolResult {
	key, ok := args["key"].(string)
	if !ok || key == "" {
		return types.ToolResult{Status: "error", Output: "Missing 'key' argument"}
	}

	res := t.ctrl.PressKey(key)
	if res.Status != "error" {
		res.Output += "\n\n" + t.ctrl.Screenshot().Output
	}
	return res
}

// ---- Register All Browser Tools ----

// RegisterBrowserTools attaches browser control capabilities to the tool registry.
func RegisterBrowserTools(r *Registry, ctrl *browser.Controller) {
	r.Register(&BrowserNavigateTool{ctrl: ctrl})
	r.Register(&BrowserClickTool{ctrl: ctrl})
	r.Register(&BrowserTypeTool{ctrl: ctrl})
	r.Register(&BrowserExtractDOMTool{ctrl: ctrl})

	// New Tools
	r.Register(&BrowserScrollTool{ctrl: ctrl})
	r.Register(&BrowserWaitForSelectorTool{ctrl: ctrl})
	r.Register(&BrowserEvalJSTool{ctrl: ctrl})
	r.Register(&BrowserSelectOptionTool{ctrl: ctrl})
	r.Register(&BrowserPressKeyTool{ctrl: ctrl})
}
