package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mebot/types"
)

// ---- get_current_time ----

type GetCurrentTimeTool struct{}

func (t *GetCurrentTimeTool) Name() string        { return "get_current_time" }
func (t *GetCurrentTimeTool) Description() string { return "Get the current date and time" }

func (t *GetCurrentTimeTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "get_current_time",
		Description: "Get the current date and time in a specific timezone. Returns ISO 8601 formatted timestamp.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"timezone": {
					Type:        "STRING",
					Description: "IANA timezone name, e.g. 'Asia/Kolkata', 'America/New_York', 'UTC'. Defaults to UTC.",
				},
			},
		},
	}
}

func (t *GetCurrentTimeTool) Execute(args map[string]any) types.ToolResult {
	tz := "UTC"
	if v, ok := args["timezone"].(string); ok && v != "" {
		tz = v
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Invalid timezone '%s': %v", tz, err),
		}
	}

	now := time.Now().In(loc)
	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Current time in %s: %s", tz, now.Format(time.RFC3339)),
	}
}

// ---- web_request ----

type WebRequestTool struct{}

func (t *WebRequestTool) Name() string        { return "web_request" }
func (t *WebRequestTool) Description() string { return "Make an HTTP request to a URL" }

func (t *WebRequestTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "web_request",
		Description: "Make an HTTP request to a URL and return the status code and response body. Useful for checking if a website is up, fetching API data, etc.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"url": {
					Type:        "STRING",
					Description: "The URL to request",
				},
				"method": {
					Type:        "STRING",
					Description: "HTTP method: GET, POST, PUT, DELETE. Defaults to GET.",
				},
			},
			Required: []string{"url"},
		},
	}
}

func (t *WebRequestTool) Execute(args map[string]any) types.ToolResult {
	url, _ := args["url"].(string)
	if url == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: url"}
	}

	method := "GET"
	if m, ok := args["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to create request: %v", err)}
	}
	req.Header.Set("User-Agent", "MeBot/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Request failed: %v", err)}
	}
	defer resp.Body.Close()

	// Read body (truncate to 2000 chars)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to read response: %v", err)}
	}

	bodyStr := string(body)
	if len(bodyStr) > 2000 {
		bodyStr = bodyStr[:2000] + "\n... (truncated)"
	}

	output := fmt.Sprintf("HTTP %d %s\n\n%s", resp.StatusCode, resp.Status, bodyStr)

	return types.ToolResult{
		Status: "success",
		Output: output,
	}
}
