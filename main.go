package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"mebot/browser"
	"mebot/config"
	"mebot/engine"
	"mebot/llm"
	"mebot/tools"
	ws "mebot/websocket"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("🤖 MeBot starting...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Config error: %v", err)
		log.Println("Set GEMINI_API_KEY environment variable or create ~/.config/mebot/config.yaml")
		os.Exit(1)
	}
	log.Printf("Config loaded: provider=%s, model=%s, port=%d", cfg.LLMProvider, cfg.Model, cfg.Port)

	// Initialize Playwright Browser Controller (Headed mode locally for dev visibility)
	ctrl, err := browser.NewController(false)
	if err != nil {
		log.Fatalf("Failed to initialize Playwright browser: %v", err)
	}
	defer ctrl.Close()
	log.Println("Browser controller initialized")

	// Initialize LLM client
	var llmClient llm.Provider
	if cfg.LLMProvider == "openrouter" {
		llmClient, err = llm.NewOpenRouterProvider(cfg.OpenRouterAPIKeys, cfg.Model)
	} else if cfg.LLMProvider == "scraper" {
		llmClient, err = llm.NewScraperProvider(ctrl)
	} else if cfg.LLMProvider == "ollama" {
		llmClient, err = llm.NewOllamaProvider(cfg.OllamaEndpoint, cfg.Model)
	} else if cfg.LLMProvider == "nvidia" {
		llmClient, err = llm.NewNvidiaProvider(cfg.NvidiaAPIKeys, cfg.Model)
	} else {
		llmClient, err = llm.NewGeminiProvider(cfg.GeminiAPIKeys, cfg.Model)
	}

	if err != nil {
		log.Fatalf("Failed to create %s LLM provider: %v", cfg.LLMProvider, err)
	}
	log.Printf("%s LLM client initialized", cfg.LLMProvider)

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()
	toolRegistry.RegisterDefaults()
	tools.RegisterBrowserTools(toolRegistry, ctrl) // Add browser capability!
	log.Println("Tool registry initialized")

	// Initialize engine
	eng := engine.NewEngine(cfg, llmClient, toolRegistry)
	log.Println("Engine initialized")

	// Initialize Google API Auth
	googleAPI := "mebot/api/google" // avoid unused import error if not implemented yet
	_ = googleAPI
	importGoogle := func() {
		// we use an anonymous func because we are injecting code string directly
	}
	_ = importGoogle

	// Set up HTTP routes
	wsHandler := ws.NewHandler(eng)

	// WebSocket endpoint
	http.HandleFunc("/ws", wsHandler.ServeWS)

	// Google OAuth Callback
	http.HandleFunc("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		// In an ideal world, call google.HandleCallback(code)
		fmt.Fprintf(w, "Google Auth Code received! Check your terminal for further instructions: %s", code)
	})

	// Serve frontend static files
	frontendDir := "./frontend/dist"
	if _, err := os.Stat(frontendDir); err == nil {
		fs := http.FileServer(http.Dir(frontendDir))
		http.Handle("/", fs)
		log.Printf("Serving frontend from %s", frontendDir)
	} else {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>MeBot</title></head>
<body style="background:#111;color:#fff;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center">
<h1>🤖 MeBot</h1>
<p>Backend is running. Build the frontend with <code>cd frontend && npm run build</code></p>
<p>WebSocket endpoint: <code>/ws</code></p>
</div></body></html>`)
		})
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("🚀 MeBot listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
