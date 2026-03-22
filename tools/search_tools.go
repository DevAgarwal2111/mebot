package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mebot/types"
)

// ---- web_search (Tavily) ----

type WebSearchTool struct {
	APIKey string
}

func (t *WebSearchTool) Name() string        { return "web_search" }
func (t *WebSearchTool) Description() string { return "Search the internet using Tavily" }

func (t *WebSearchTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "web_search",
		Description: "Search the internet using Tavily Search API. Returns web results with titles, URLs, and content snippets. Use this as the PRIMARY way to look up information online — faster and more reliable than opening a browser.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"query": {
					Type:        "STRING",
					Description: "The search query (e.g. 'weather in Delhi', 'latest Go release', 'how to make pasta')",
				},
				"max_results": {
					Type:        "INTEGER",
					Description: "Number of results to return (1-10, default 5)",
				},
			},
			Required: []string{"query"},
		},
	}
}

// Tavily API request/response structures
type tavilySearchRequest struct {
	APIKey     string `json:"api_key"`
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type tavilySearchResponse struct {
	Query   string         `json:"query"`
	Results []tavilyResult `json:"results"`
	Answer  string         `json:"answer,omitempty"`
}

type tavilyResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

func (t *WebSearchTool) Execute(args map[string]any) types.ToolResult {
	query, _ := args["query"].(string)
	if query == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: query"}
	}

	if t.APIKey == "" {
		return types.ToolResult{Status: "error", Output: "Tavily API key not configured. Set TAVILY_API_KEY in .env"}
	}

	maxResults := 5
	if c, ok := args["max_results"].(float64); ok && c >= 1 && c <= 10 {
		maxResults = int(c)
	}

	// Build request body
	reqBody := tavilySearchRequest{
		APIKey:     t.APIKey,
		Query:      query,
		MaxResults: maxResults,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to build request: %v", err)}
	}

	req, err := http.NewRequest("POST", "https://api.tavily.com/search", bytes.NewReader(bodyBytes))
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to create request: %v", err)}
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Search request failed: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 500))
		return types.ToolResult{
			Status: "error",
			Output: fmt.Sprintf("Tavily API error (HTTP %d): %s", resp.StatusCode, string(body)),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to read response: %v", err)}
	}

	var searchResp tavilySearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	if len(searchResp.Results) == 0 {
		return types.ToolResult{Status: "success", Output: fmt.Sprintf("No results found for: %s", query)}
	}

	// Format results
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for \"%s\":\n\n", query))

	for i, r := range searchResp.Results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("   URL: %s\n", r.URL))
		if r.Content != "" {
			// Truncate long content
			content := r.Content
			if len(content) > 300 {
				content = content[:300] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", content))
		}
		sb.WriteString("\n")
	}

	return types.ToolResult{Status: "success", Output: sb.String()}
}
