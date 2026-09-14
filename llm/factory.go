package llm

import (
	"fmt"

	"mebot/browser"
	"mebot/config"
)

// NewProvider is the composition-root factory for model backends. The engine
// only receives the Provider interface and never needs to know which backend
// is active.
func NewProvider(cfg *config.Config, browserController *browser.Controller) (Provider, error) {
	switch cfg.LLMProvider {
	case "gemini":
		return NewGeminiProvider(cfg.GeminiAPIKeys, cfg.Model)
	case "foundry":
		return NewFoundryProvider(cfg.FoundryEndpoint, cfg.FoundryAPIKey, cfg.Model)
	case "openrouter":
		return NewOpenRouterProvider(cfg.OpenRouterAPIKeys, cfg.Model)
	case "scraper":
		return NewScraperProvider(browserController)
	case "ollama":
		return NewOllamaProvider(cfg.OllamaEndpoint, cfg.Model)
	case "nvidia":
		return NewNvidiaProvider(cfg.NvidiaAPIKeys, cfg.Model)
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", cfg.LLMProvider)
	}
}