package skills

import (
	"fmt"
	"strings"
)

// Router selects relevant skills for a given user message.
type Router struct {
	loader *Loader
}

// NewRouter creates a skill router backed by the given loader.
func NewRouter(loader *Loader) *Router {
	return &Router{loader: loader}
}

// Route returns skills that match the user message based on trigger keywords.
// It also always returns skills with empty triggers (global skills).
func (r *Router) Route(userMessage string) []*Skill {
	allSkills := r.loader.GetAll()
	msgLower := strings.ToLower(userMessage)

	var matched []*Skill
	for _, skill := range allSkills {
		// Skills with no triggers are always injected (global skills)
		if len(skill.Triggers) == 0 {
			matched = append(matched, skill)
			continue
		}

		// Check if any trigger keyword matches
		for _, trigger := range skill.Triggers {
			if strings.Contains(msgLower, strings.ToLower(trigger)) {
				matched = append(matched, skill)
				break
			}
		}
	}

	return matched
}

// FormatForPrompt formats matched skills into a string to inject into the system prompt.
func (r *Router) FormatForPrompt(skills []*Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n[ACTIVE SKILLS FOR THIS TURN]\n")
	sb.WriteString("The following skills are relevant to this conversation. Follow their instructions:\n\n")

	for i, skill := range skills {
		sb.WriteString(fmt.Sprintf("--- SKILL: %s ---\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("Description: %s\n", skill.Description))
		}
		sb.WriteString(skill.Content)
		if i < len(skills)-1 {
			sb.WriteString("\n\n")
		}
	}

	sb.WriteString("\n[END OF ACTIVE SKILLS]\n")
	return sb.String()
}

// Loader returns the underlying loader (for skill tools to access).
func (r *Router) GetLoader() *Loader {
	return r.loader
}
