package tools

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func NewSkillsListSkill(tc *ToolContext, skillSvc services.AgentSkillManager) *tool.Entry {
	return &tool.Entry{
		Name:           "skills_list",
		Toolset:        "skills",
		Description:    "List all available skills in this workspace with minimal metadata (name, description, category). Use skill_view(name) to load full content.",
		Emoji:          "📚",
		MaxResultChars: 8 * 1024,
		Schema: core.ToolDefinition{
			Name:        "skills_list",
			Description: "List available skills in this workspace",
			Parameters:  skillsListSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Category string `json:"category,omitempty"`
			}
			_ = json.Unmarshal(raw, &args)

			skills, err := skillSvc.List(ctx, tc.Workspace.ID, false)
			if err != nil {
				return tool.Error("Failed to list skills: " + err.Error()), nil
			}

			items := make([]map[string]any, 0, len(skills))
			categories := make(map[string]bool)
			for _, s := range skills {
				if args.Category != "" && s.Category != args.Category {
					continue
				}
				items = append(items, map[string]any{
					"name":        s.Name,
					"description": s.Description,
					"category":    s.Category,
					"status":      s.Status,
				})
				if s.Category != "" {
					categories[s.Category] = true
				}
			}

			catList := make([]string, 0, len(categories))
			for c := range categories {
				catList = append(catList, c)
			}

			return tool.Result(map[string]any{
				"success":    true,
				"skills":     items,
				"categories": catList,
				"count":      len(items),
				"hint":       "Use skill_view(name) to see full content, tags, and linked files",
			}), nil
		},
	}
}

func skillsListSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"category": {
				"type": "string",
				"description": "Optional category filter (e.g., 'devops', 'data-science')"
			}
		}
	}`))
}
