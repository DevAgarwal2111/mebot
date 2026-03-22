package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"mebot/skills"
)

// SkillsAPI handles REST endpoints for skill management from the frontend.
type SkillsAPI struct {
	router *skills.Router
}

// NewSkillsAPI creates a new skills API handler.
func NewSkillsAPI(router *skills.Router) *SkillsAPI {
	return &SkillsAPI{router: router}
}

// RegisterRoutes registers /api/skills endpoints.
func (api *SkillsAPI) RegisterRoutes() {
	http.HandleFunc("/api/skills", api.handleSkills)
	http.HandleFunc("/api/skills/", api.handleSkillByName)
}

// SkillResponse is the JSON shape for a skill in API responses.
type SkillResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Triggers    []string `json:"triggers"`
	CronExpr    string   `json:"cron,omitempty"`
	Content     string   `json:"content"`
	IsBuiltIn   bool     `json:"is_builtin"`
}

// CreateSkillRequest is the JSON body for creating a skill.
type CreateSkillRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"` // Full markdown content (with or without frontmatter)
}

func (api *SkillsAPI) handleSkills(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case "GET":
		api.listSkills(w, r)
	case "POST":
		api.createSkill(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (api *SkillsAPI) handleSkillByName(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Extract skill name from URL: /api/skills/{name}
	name := strings.TrimPrefix(r.URL.Path, "/api/skills/")
	if name == "" {
		http.Error(w, "Skill name is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "DELETE":
		api.deleteSkill(w, r, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (api *SkillsAPI) listSkills(w http.ResponseWriter, r *http.Request) {
	allSkills := api.router.GetLoader().GetAll()

	resp := make([]SkillResponse, 0, len(allSkills))
	for _, s := range allSkills {
		resp = append(resp, SkillResponse{
			Name:        s.Name,
			Description: s.Description,
			Triggers:    s.Triggers,
			CronExpr:    s.CronExpr,
			Content:     s.Content,
			IsBuiltIn:   s.IsBuiltIn,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (api *SkillsAPI) createSkill(w http.ResponseWriter, r *http.Request) {
	var req CreateSkillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Content == "" {
		http.Error(w, "Name and content are required", http.StatusBadRequest)
		return
	}

	if err := api.router.GetLoader().AddSkill(req.Name, req.Content); err != nil {
		log.Printf("[API] Failed to create skill: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "created",
		"name":   req.Name,
	})
}

func (api *SkillsAPI) deleteSkill(w http.ResponseWriter, r *http.Request, name string) {
	if err := api.router.GetLoader().RemoveSkill(name); err != nil {
		log.Printf("[API] Failed to delete skill: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "deleted",
		"name":   name,
	})
}
