package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"path/filepath"

	"mebot/agents"
	"mebot/api"
	googleauth "mebot/api/google"
	"mebot/browser"
	"mebot/config"
	"mebot/engine"
	"mebot/llm"
	"mebot/skills"
	"mebot/tools"
	"mebot/types"
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

	// Initialize Google OAuth (Gmail, Calendar)
	if err := googleauth.InitAuth(cfg); err != nil {
		log.Printf("Warning: Google Auth init failed: %v", err)
	}

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

	// Initialize Skill System
	// Use the project root directory as base for skills/builtin and skills/user
	skillLoader, err := skills.NewLoader(".")
	if err != nil {
		log.Fatalf("Failed to initialize skill loader: %v", err)
	}
	if err := skillLoader.LoadAll(); err != nil {
		log.Printf("Warning: failed to load some skills: %v", err)
	}
	skillRouter := skills.NewRouter(skillLoader)
	log.Println("Skill system initialized")

	// Initialize agent loader
	agentDir := filepath.Join(cfg.SessionDir, "agents")
	agentLoader := agents.NewLoader(agentDir)
	if err := agentLoader.LoadAll(); err != nil {
		log.Printf("Warning: failed to load agents: %v", err)
	}
	log.Println("Agent system initialized")

	// Create a broadcast closure to break the initialization circle between tools, engine, and websockets
	var broadcastFunc func(types.WSEvent)
	broadcast := func(e types.WSEvent) {
		if broadcastFunc != nil {
			broadcastFunc(e)
		} else {
			log.Printf("[Broadcast] Dropped event (no handler yet): %v", e.Type)
		}
	}

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()
	toolRegistry.RegisterDefaults(skillRouter, cfg, broadcast)
	tools.RegisterBrowserTools(toolRegistry, ctrl)
	log.Println("Tool registry initialized")

	// Initialize engine (now with skill router and agent loader)
	eng := engine.NewEngine(cfg, llmClient, toolRegistry, skillRouter, agentLoader)
	log.Println("Engine initialized")

	// Initialize Skill Scheduler (cron-based triggers)
	scheduler := skills.NewScheduler(skillLoader, func(skill *skills.Skill) {
		log.Printf("[Scheduler] Auto-triggering skill: %s", skill.Name)
		broadcast(types.WSEvent{
			Type:    "assistant",
			Content: fmt.Sprintf("⏰ **Scheduled Skill Triggered:** %s", skill.Name),
		})
	})
	scheduler.Start()
	defer scheduler.Stop()

	// Ensure the scheduler picks up dynamically created/edited cron skills
	skillLoader.SetOnChange(func() {
		log.Println("[Main] Skill change detected, reloading scheduler")
		scheduler.Reload()
	})

	// Set up HTTP routes
	wsHandler := ws.NewHandler(eng)
	broadcastFunc = wsHandler.Broadcast // wire up the actual broadcast method

	// WebSocket endpoint
	http.HandleFunc("/ws", wsHandler.ServeWS)

	// Skills REST API
	skillsAPI := api.NewSkillsAPI(skillRouter)
	skillsAPI.RegisterRoutes()
	log.Println("Skills REST API registered (/api/skills)")

	// Agents REST API
	agentsAPI := api.NewAgentsAPI(agentLoader)
	agentsAPI.RegisterRoutes()
	log.Println("Agents REST API registered (/api/agents)")

	// Google OAuth: redirect to consent screen
	http.HandleFunc("/auth/google", func(w http.ResponseWriter, r *http.Request) {
		authURL := googleauth.GetAuthURL()
		if authURL == "" {
			http.Error(w, "Google OAuth not configured", http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	})

	// Google OAuth Callback
	http.HandleFunc("/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		tok, err := googleauth.HandleCallback(code)
		if err != nil {
			log.Printf("Google OAuth callback error: %v", err)
			http.Error(w, fmt.Sprintf("Failed to exchange token: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Google OAuth token saved successfully! Expiry: %v", tok.Expiry)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body style="background:#111;color:#0f0;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
<div style="text-align:center"><h1>Google Account Connected!</h1><p>You can close this tab and return to MeBot.</p></div></body></html>`)
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
