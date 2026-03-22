package tools

import (
	"fmt"
	"strings"

	"mebot/skills"
	"mebot/types"
)

// ---- create_skill ----

type CreateSkillTool struct {
	Router *skills.Router
}

func (t *CreateSkillTool) Name() string        { return "create_skill" }
func (t *CreateSkillTool) Description() string { return "Create a new skill/memory for future reference" }

func (t *CreateSkillTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name: "create_skill",
		Description: `Create a new skill (a persistent memory/instruction) that will be available in future conversations.
Use this proactively when you learn something worth remembering: user preferences, important contacts, workflows, routines, facts about the user, etc.
The skill content should be a valid markdown file with YAML frontmatter.`,
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"name": {
					Type:        "STRING",
					Description: "A short, descriptive name for the skill (e.g. 'user_coffee_preference', 'manager_contact', 'morning_routine')",
				},
				"description": {
					Type:        "STRING",
					Description: "A one-line description of what this skill remembers or does",
				},
				"triggers": {
					Type:        "STRING",
					Description: "Comma-separated keywords that should activate this skill (e.g. 'coffee,caffeine,drink' or 'manager,boss,email boss')",
				},
				"content": {
					Type:        "STRING",
					Description: "The skill instructions/knowledge in markdown. This is the body that gets injected when the skill is activated.",
				},
				"cron": {
					Type:        "STRING",
					Description: "Optional cron expression for scheduled skills (e.g. '0 9 * * *' for 9am daily). Leave empty for non-scheduled skills.",
				},
			},
			Required: []string{"name", "description", "content"},
		},
	}
}

func (t *CreateSkillTool) Execute(args map[string]any) types.ToolResult {
	name, _ := args["name"].(string)
	description, _ := args["description"].(string)
	content, _ := args["content"].(string)
	triggers, _ := args["triggers"].(string)
	cronExpr, _ := args["cron"].(string)

	if name == "" || content == "" {
		return types.ToolResult{Status: "error", Output: "Missing required arguments: name and content"}
	}

	// Build the full markdown file with frontmatter
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", name))
	sb.WriteString(fmt.Sprintf("description: %s\n", description))
	if triggers != "" {
		triggerList := strings.Split(triggers, ",")
		sb.WriteString("triggers:\n")
		for _, tr := range triggerList {
			tr = strings.TrimSpace(tr)
			if tr != "" {
				sb.WriteString(fmt.Sprintf("  - \"%s\"\n", tr))
			}
		}
	}
	if cronExpr != "" {
		sb.WriteString(fmt.Sprintf("cron: \"%s\"\n", cronExpr))
	}
	sb.WriteString("---\n\n")
	sb.WriteString(content)

	err := t.Router.GetLoader().AddSkill(name, sb.String())
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to create skill: %v", err)}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Skill '%s' created successfully. It will be available in future conversations when triggered by: %s", name, triggers),
	}
}

// ---- update_skill ----

type UpdateSkillTool struct {
	Router *skills.Router
}

func (t *UpdateSkillTool) Name() string        { return "update_skill" }
func (t *UpdateSkillTool) Description() string { return "Update an existing skill/memory" }

func (t *UpdateSkillTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "update_skill",
		Description: "Update the content of an existing user-created skill. Use this when you learn new information that should update an existing memory/skill.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"name": {
					Type:        "STRING",
					Description: "Name of the skill to update",
				},
				"content": {
					Type:        "STRING",
					Description: "The complete new content for the skill file (including YAML frontmatter and markdown body)",
				},
			},
			Required: []string{"name", "content"},
		},
	}
}

func (t *UpdateSkillTool) Execute(args map[string]any) types.ToolResult {
	name, _ := args["name"].(string)
	content, _ := args["content"].(string)

	if name == "" || content == "" {
		return types.ToolResult{Status: "error", Output: "Missing required arguments: name and content"}
	}

	err := t.Router.GetLoader().UpdateSkill(name, content)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to update skill: %v", err)}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Skill '%s' updated successfully.", name),
	}
}

// ---- list_skills ----

type ListSkillsTool struct {
	Router *skills.Router
}

func (t *ListSkillsTool) Name() string        { return "list_skills" }
func (t *ListSkillsTool) Description() string { return "List all loaded skills/memories" }

func (t *ListSkillsTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "list_skills",
		Description: "Returns a list of all loaded skills (both built-in and user-created), including their names, descriptions, and trigger keywords.",
		Parameters: &types.Schema{
			Type:       "OBJECT",
			Properties: map[string]*types.Schema{},
		},
	}
}

func (t *ListSkillsTool) Execute(args map[string]any) types.ToolResult {
	allSkills := t.Router.GetLoader().GetAll()

	if len(allSkills) == 0 {
		return types.ToolResult{Status: "success", Output: "No skills loaded."}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Loaded %d skills:\n\n", len(allSkills)))

	for _, s := range allSkills {
		badge := "user"
		if s.IsBuiltIn {
			badge = "built-in"
		}
		sb.WriteString(fmt.Sprintf("• %s [%s]\n", s.Name, badge))
		if s.Description != "" {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", s.Description))
		}
		if len(s.Triggers) > 0 {
			sb.WriteString(fmt.Sprintf("  Triggers: %s\n", strings.Join(s.Triggers, ", ")))
		}
		if s.CronExpr != "" {
			sb.WriteString(fmt.Sprintf("  Cron: %s\n", s.CronExpr))
		}
		sb.WriteString("\n")
	}

	return types.ToolResult{Status: "success", Output: sb.String()}
}

// ---- delete_skill ----

type DeleteSkillTool struct {
	Router *skills.Router
}

func (t *DeleteSkillTool) Name() string        { return "delete_skill" }
func (t *DeleteSkillTool) Description() string { return "Delete a user-created skill/memory" }

func (t *DeleteSkillTool) Declaration() *types.ToolDeclaration {
	return &types.ToolDeclaration{
		Name:        "delete_skill",
		Description: "Delete a user-created skill by name. Built-in skills cannot be deleted.",
		Parameters: &types.Schema{
			Type: "OBJECT",
			Properties: map[string]*types.Schema{
				"name": {
					Type:        "STRING",
					Description: "Name of the skill to delete",
				},
			},
			Required: []string{"name"},
		},
	}
}

func (t *DeleteSkillTool) Execute(args map[string]any) types.ToolResult {
	name, _ := args["name"].(string)
	if name == "" {
		return types.ToolResult{Status: "error", Output: "Missing required argument: name"}
	}

	err := t.Router.GetLoader().RemoveSkill(name)
	if err != nil {
		return types.ToolResult{Status: "error", Output: fmt.Sprintf("Failed to delete skill: %v", err)}
	}

	return types.ToolResult{
		Status: "success",
		Output: fmt.Sprintf("Skill '%s' deleted successfully.", name),
	}
}
