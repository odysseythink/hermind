package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func NewSkillViewSkill(tc *ToolContext, skillSvc services.AgentSkillManager) *tool.Entry {
	return &tool.Entry{
		Name:           "skill_view",
		Toolset:        "skills",
		Description:    "View the full content of a skill or a specific supporting file within it.",
		Emoji:          "📖",
		MaxResultChars: 16 * 1024,
		Schema: core.ToolDefinition{
			Name:        "skill_view",
			Description: "View a skill's full SKILL.md content or a specific supporting file",
			Parameters:  skillViewSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Name     string `json:"name"`
				FilePath string `json:"file_path,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if args.Name == "" {
				return tool.Error("name is required"), nil
			}

			wsID := tc.Workspace.ID
			skillSlug := slugifyForLookup(args.Name)

			skill, err := skillSvc.GetBySlug(ctx, wsID, skillSlug)
			if err != nil {
				return tool.Error(fmt.Sprintf("Skill '%s' not found.", args.Name)), nil
			}

			// Bump view telemetry only after confirming skill exists
			_ = skillSvc.BumpView(ctx, wsID, skillSlug)

			// If file_path is provided, load supporting file
			if args.FilePath != "" {
				file, err := skillSvc.GetFile(ctx, skill.ID, args.FilePath)
				if err != nil {
					// List available files for self-correction
					files, _ := skillSvc.ListFiles(ctx, skill.ID)
					available := make([]string, 0, len(files))
					for _, f := range files {
						available = append(available, f.FilePath)
					}
					return tool.Result(map[string]any{
						"success":         false,
						"error":           fmt.Sprintf("File '%s' not found in skill '%s'.", args.FilePath, args.Name),
						"available_files": available,
					}), nil
				}
				return tool.Result(map[string]any{
					"success":   true,
					"name":      skill.Name,
					"file_path": file.FilePath,
					"content":   file.Content,
				}), nil
			}

			// Return full SKILL.md (frontmatter + body)
			fullContent := "---\n" + skill.Frontmatter + "\n---\n\n" + skill.Content

			// List linked files
			files, _ := skillSvc.ListFiles(ctx, skill.ID)
			linkedFiles := make([]string, 0, len(files))
			for _, f := range files {
				linkedFiles = append(linkedFiles, f.FilePath)
			}

			return tool.Result(map[string]any{
				"success":      true,
				"name":         skill.Name,
				"description":  skill.Description,
				"category":     skill.Category,
				"status":       skill.Status,
				"content":      fullContent,
				"linked_files": linkedFiles,
				"hint":         "Load a linked file with skill_view(name, file_path='references/...')",
			}), nil
		},
	}
}

func skillViewSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"description": "Name of the skill to view."
			},
			"file_path": {
				"type": "string",
				"description": "Optional path to a specific file within the skill (e.g., 'references/api.md'). If omitted, returns the main SKILL.md."
			}
		},
		"required": ["name"]
	}`))
}
