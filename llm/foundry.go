package llm

import (
	"fmt"
	"strings"
)

// FoundryProvider uses the Microsoft Foundry Models v1 OpenAI-compatible API.
// The model value is the Foundry deployment name, not necessarily the catalog
// model name.
type FoundryProvider struct {
	*OpenRouterProvider
}

// NewFoundryProvider creates a Foundry v1 provider using API-key auth.
func NewFoundryProvider(endpoint, apiKey, model string) (*FoundryProvider, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("Foundry endpoint is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("Foundry API key is required")
	}

	baseURL := strings.TrimRight(endpoint, "/")
	if !strings.HasSuffix(baseURL, "/openai/v1") {
		baseURL += "/openai/v1"
	}

	provider, err := NewOpenAICompatibleProvider([]string{apiKey}, baseURL, model)
	if err != nil {
		return nil, err
	}

	return &FoundryProvider{OpenRouterProvider: provider}, nil
}