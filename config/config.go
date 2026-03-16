package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	LLMProvider        string   `yaml:"llm_provider"`
	OllamaEndpoint     string   `yaml:"ollama_endpoint"`
	NvidiaAPIKeys      []string `yaml:"nvidia_api_keys"`
	GeminiAPIKeys      []string `yaml:"llm_api_keys"`
	OpenRouterAPIKeys  []string `yaml:"openrouter_api_keys"`
	Model              string   `yaml:"llm_model"`
	Port               int      `yaml:"port"`
	MaxToolIterations  int      `yaml:"max_tool_iterations"`
	ToolTimeoutSecs    int      `yaml:"tool_timeout_seconds"`
	SessionDir         string   `yaml:"session_dir"`
	StreamFPS          int      `yaml:"stream_fps"`
	MaxHistoryMessages int      `yaml:"max_history_messages"`
	GoogleClientID     string   `yaml:"google_client_id"`
	GoogleClientSecret string   `yaml:"google_client_secret"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		LLMProvider:        "gemini",
		Model:              "gemini-2.5-flash",
		Port:               8085,
		MaxToolIterations:  100, // increased to allow many iterations
		ToolTimeoutSecs:    30,
		SessionDir:         "~/.mebot/sessions",
		StreamFPS:          2,
		MaxHistoryMessages: 100, // increased context history
	}
}

// Load reads config from ~/.config/mebot/config.yaml, then overlays env vars.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Load .env file if it exists
	_ = godotenv.Load()

	// Environment variable overrides
	if provider := os.Getenv("LLM_PROVIDER"); provider != "" {
		cfg.LLMProvider = strings.ToLower(provider)
	}

	if keysStr := os.Getenv("GEMINI_API_KEYS"); keysStr != "" {
		parts := strings.Split(keysStr, ",")
		for _, k := range parts {
			if trimmed := strings.TrimSpace(k); trimmed != "" {
				cfg.GeminiAPIKeys = append(cfg.GeminiAPIKeys, trimmed)
			}
		}
	}
	// Fallback to singular key
	if len(cfg.GeminiAPIKeys) == 0 {
		if key := os.Getenv("GEMINI_API_KEY"); key != "" {
			cfg.GeminiAPIKeys = []string{key}
		}
	}
	if keysStr := os.Getenv("OPENROUTER_API_KEYS"); keysStr != "" {
		parts := strings.Split(keysStr, ",")
		for _, k := range parts {
			if trimmed := strings.TrimSpace(k); trimmed != "" {
				cfg.OpenRouterAPIKeys = append(cfg.OpenRouterAPIKeys, trimmed)
			}
		}
	}
	if len(cfg.OpenRouterAPIKeys) == 0 {
		if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
			cfg.OpenRouterAPIKeys = []string{key}
		}
	}
	if keysStr := os.Getenv("NVIDIA_API_KEYS"); keysStr != "" {
		parts := strings.Split(keysStr, ",")
		for _, k := range parts {
			if trimmed := strings.TrimSpace(k); trimmed != "" {
				cfg.NvidiaAPIKeys = append(cfg.NvidiaAPIKeys, trimmed)
			}
		}
	}
	if len(cfg.NvidiaAPIKeys) == 0 {
		if key := os.Getenv("NVIDIA_API_KEY"); key != "" {
			cfg.NvidiaAPIKeys = []string{key}
		}
	}

	if model := os.Getenv("MEBOT_MODEL"); model != "" {
		cfg.Model = model
	}
	if port := os.Getenv("MEBOT_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}
	if clientID := os.Getenv("GOOGLE_CLIENT_ID"); clientID != "" {
		cfg.GoogleClientID = clientID
	}
	if clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET"); clientSecret != "" {
		cfg.GoogleClientSecret = clientSecret
	}
	if endpoint := os.Getenv("OLLAMA_ENDPOINT"); endpoint != "" {
		cfg.OllamaEndpoint = endpoint
	}
	if maxHist := os.Getenv("MEBOT_MAX_HISTORY"); maxHist != "" {
		if m, err := strconv.Atoi(maxHist); err == nil {
			cfg.MaxHistoryMessages = m
		}
	}

	// Validate required fields
	if cfg.LLMProvider == "gemini" && len(cfg.GeminiAPIKeys) == 0 {
		return nil, fmt.Errorf("GEMINI_API_KEYS (comma-separated list) or GEMINI_API_KEY is required in .env for gemini provider")
	}
	if cfg.LLMProvider == "openrouter" && len(cfg.OpenRouterAPIKeys) == 0 {
		return nil, fmt.Errorf("OPENROUTER_API_KEYS or OPENROUTER_API_KEY is required in .env for openrouter provider")
	}
	if cfg.LLMProvider == "nvidia" && len(cfg.NvidiaAPIKeys) == 0 {
		return nil, fmt.Errorf("NVIDIA_API_KEYS or NVIDIA_API_KEY is required in .env for nvidia provider")
	}

	return cfg, nil
}
