package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"mebot/agents"
)

type AgentsAPI struct {
	loader *agents.Loader
}

func NewAgentsAPI(loader *agents.Loader) *AgentsAPI {
	return &AgentsAPI{loader: loader}
}

func (app *AgentsAPI) RegisterRoutes() {
	http.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Setup CORS if needed here
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		switch r.Method {
		case http.MethodGet:
			app.handleGetAgents(w, r)
		case http.MethodPost:
			app.handleSaveAgent(w, r)
		case http.MethodDelete:
			app.handleDeleteAgent(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func (app *AgentsAPI) handleGetAgents(w http.ResponseWriter, r *http.Request) {
	list := app.loader.GetAll()
	json.NewEncoder(w).Encode(list)
}

func (app *AgentsAPI) handleSaveAgent(w http.ResponseWriter, r *http.Request) {
	var agent agents.Agent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Basic validation
	if strings.TrimSpace(agent.Name) == "" || strings.TrimSpace(agent.Provider) == "" || strings.TrimSpace(agent.Model) == "" {
		http.Error(w, "Name, Provider, and Model are required", http.StatusBadRequest)
		return
	}

	if err := app.loader.Save(&agent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": agent.ID})
}

func (app *AgentsAPI) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "agent ID required", http.StatusBadRequest)
		return
	}

	if err := app.loader.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
