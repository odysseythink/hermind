package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gosimple/slug"
	"github.com/odysseythink/hermind/backend/internal/dto"
	"github.com/odysseythink/hermind/backend/internal/models"
	"github.com/odysseythink/hermind/backend/internal/services"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func NewSkillManageSkill(tc *ToolContext, skillSvc services.AgentSkillManager) *tool.Entry {
	return &tool.Entry{
		Name:           "skill_manage",
		Toolset:        "skills",
		Description:    "Manage procedural memory skills (create, update, delete). Skills are reusable approaches for recurring task types.",
		Emoji:          "📝",
		MaxResultChars: 8 * 1024,
		Schema: core.ToolDefinition{
			Name:        "skill_manage",
			Description: "Manage skills (create, update, delete). Skills are your procedural memory — reusable approaches for recurring task types.",
			Parameters:  skillManageSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args skillManageArgs
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			return handleSkillManage(ctx, tc, skillSvc, args)
		},
	}
}

type skillManageArgs struct {
	Action       string `json:"action"`
	Name         string `json:"name"`
	Content      string `json:"content,omitempty"`
	Category     string `json:"category,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
	FileContent  string `json:"file_content,omitempty"`
	OldString    string `json:"old_string,omitempty"`
	NewString    string `json:"new_string,omitempty"`
	ReplaceAll   bool   `json:"replace_all,omitempty"`
	AbsorbedInto string `json:"absorbed_into,omitempty"`
}

func handleSkillManage(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, args skillManageArgs) (string, error) {
	if args.Name == "" {
		return tool.Error("name is required"), nil
	}

	wsID := tc.Workspace.ID

	switch args.Action {
	case "create":
		return skillManageCreate(ctx, tc, skillSvc, wsID, args)
	case "edit":
		return skillManageEdit(ctx, tc, skillSvc, wsID, args)
	case "patch":
		return skillManagePatch(ctx, tc, skillSvc, wsID, args)
	case "delete":
		return skillManageDelete(ctx, tc, skillSvc, wsID, args)
	case "write_file":
		return skillManageWriteFile(ctx, tc, skillSvc, wsID, args)
	case "remove_file":
		return skillManageRemoveFile(ctx, tc, skillSvc, wsID, args)
	default:
		return tool.Error(fmt.Sprintf("Unknown action '%s'. Use: create, edit, patch, delete, write_file, remove_file", args.Action)), nil
	}
}

func skillManageCreate(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, wsID int, args skillManageArgs) (string, error) {
	if args.Content == "" {
		return tool.Error("content is required for 'create'. Provide the full SKILL.md text (frontmatter + body)."), nil
	}

	// Extract frontmatter and body
	frontmatter, body, err := splitFrontmatter(args.Content)
	if err != nil {
		return tool.Error("Failed to parse frontmatter: " + err.Error()), nil
	}

	tc.Emit(fmt.Sprintf("Creating skill '%s'...", args.Name))

	skill, err := skillSvc.Create(ctx, wsID, dto.CreateAgentSkillRequest{
		Name:        args.Name,
		Category:    args.Category,
		Content:     body,
		Frontmatter: frontmatter,
		CreatedBy:   models.AgentSkillCreatedByAgent,
	})
	if err != nil {
		return tool.Error("Failed to create skill: " + err.Error()), nil
	}

	result := map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Skill '%s' created.", skill.Name),
		"slug":     skill.Slug,
		"category": skill.Category,
		"hint":     "To add reference files, templates, or scripts, use skill_manage(action='write_file', name='" + skill.Name + "', file_path='references/example.md', file_content='...')",
	}
	if frontmatter == "" {
		result["warning"] = "No YAML frontmatter detected in content. A minimal frontmatter was auto-generated. For best results, include '---\nname: ...\ndescription: ...\n---' at the start of your content."
	}
	return tool.Result(result), nil
}

func skillManageEdit(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, wsID int, args skillManageArgs) (string, error) {
	if args.Content == "" {
		return tool.Error("content is required for 'edit'. Provide the full updated SKILL.md text."), nil
	}

	// Require approval for destructive edit
	if tc.Approval != nil {
		approved, reason := tc.Approval(ctx, "skill_manage", args, fmt.Sprintf("Replace entire SKILL.md for skill '%s'", args.Name))
		if !approved {
			return tool.Error("Tool execution rejected: " + reason), nil
		}
	}

	frontmatter, body, err := splitFrontmatter(args.Content)
	if err != nil {
		return tool.Error("Failed to parse frontmatter: " + err.Error()), nil
	}

	tc.Emit(fmt.Sprintf("Editing skill '%s'...", args.Name))

	skill, err := skillSvc.Update(ctx, wsID, slugifyForLookup(args.Name), dto.UpdateAgentSkillRequest{
		Content:     body,
		Frontmatter: frontmatter,
	})
	if err != nil {
		return tool.Error("Failed to edit skill: " + err.Error()), nil
	}

	_ = skillSvc.BumpPatch(ctx, wsID, skill.Slug)

	result := map[string]any{
		"success": true,
		"message": fmt.Sprintf("Skill '%s' updated.", skill.Name),
		"slug":    skill.Slug,
	}
	if frontmatter == "" {
		result["warning"] = "No YAML frontmatter detected in content. Existing frontmatter was preserved. For best results, include '---\nname: ...\ndescription: ...\n---' at the start of your content."
	}
	return tool.Result(result), nil
}

func skillManagePatch(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, wsID int, args skillManageArgs) (string, error) {
	if args.OldString == "" {
		return tool.Error("old_string is required for 'patch'."), nil
	}

	tc.Emit(fmt.Sprintf("Patching skill '%s'...", args.Name))

	skillSlug := slugifyForLookup(args.Name)

	if args.FilePath != "" {
		file, err := skillSvc.PatchFile(ctx, wsID, skillSlug, dto.PatchSkillFileRequest{
			FilePath:   args.FilePath,
			OldString:  args.OldString,
			NewString:  args.NewString,
			ReplaceAll: args.ReplaceAll,
		})
		if err != nil {
			return tool.Error("Failed to patch file: " + err.Error()), nil
		}
		return tool.Result(map[string]any{
			"success":   true,
			"message":   fmt.Sprintf("Patched file '%s' in skill '%s'.", file.FilePath, args.Name),
			"file_path": file.FilePath,
		}), nil
	}

	skill, err := skillSvc.Patch(ctx, wsID, skillSlug, dto.PatchAgentSkillRequest{
		OldString:  args.OldString,
		NewString:  args.NewString,
		ReplaceAll: args.ReplaceAll,
	})
	if err != nil {
		return tool.Error("Failed to patch skill: " + err.Error()), nil
	}

	return tool.Result(map[string]any{
		"success": true,
		"message": fmt.Sprintf("Patched skill '%s'.", skill.Name),
		"slug":    skill.Slug,
	}), nil
}

func skillManageDelete(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, wsID int, args skillManageArgs) (string, error) {
	// Require approval for destructive delete
	if tc.Approval != nil {
		approved, reason := tc.Approval(ctx, "skill_manage", args, fmt.Sprintf("Delete skill '%s'", args.Name))
		if !approved {
			return tool.Error("Tool execution rejected: " + reason), nil
		}
	}

	tc.Emit(fmt.Sprintf("Deleting skill '%s'...", args.Name))

	if err := skillSvc.Delete(ctx, wsID, slugifyForLookup(args.Name)); err != nil {
		return tool.Error("Failed to delete skill: " + err.Error()), nil
	}

	msg := fmt.Sprintf("Skill '%s' deleted.", args.Name)
	if args.AbsorbedInto != "" {
		msg += fmt.Sprintf(" Content absorbed into '%s'.", args.AbsorbedInto)
	}

	return tool.Result(map[string]any{
		"success": true,
		"message": msg,
	}), nil
}

func skillManageWriteFile(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, wsID int, args skillManageArgs) (string, error) {
	if args.FilePath == "" {
		return tool.Error("file_path is required for 'write_file'. Example: 'references/api-guide.md'"), nil
	}
	if args.FileContent == "" {
		return tool.Error("file_content is required for 'write_file'."), nil
	}

	// Require approval for file writes
	if tc.Approval != nil {
		approved, reason := tc.Approval(ctx, "skill_manage", args, fmt.Sprintf("Write file '%s' to skill '%s'", args.FilePath, args.Name))
		if !approved {
			return tool.Error("Tool execution rejected: " + reason), nil
		}
	}

	tc.Emit(fmt.Sprintf("Writing file '%s' to skill '%s'...", args.FilePath, args.Name))

	if err := skillSvc.WriteFile(ctx, wsID, slugifyForLookup(args.Name), dto.WriteSkillFileRequest{
		FilePath: args.FilePath,
		Content:  args.FileContent,
	}); err != nil {
		return tool.Error("Failed to write file: " + err.Error()), nil
	}

	return tool.Result(map[string]any{
		"success": true,
		"message": fmt.Sprintf("File '%s' written to skill '%s'.", args.FilePath, args.Name),
	}), nil
}

func skillManageRemoveFile(ctx context.Context, tc *ToolContext, skillSvc services.AgentSkillManager, wsID int, args skillManageArgs) (string, error) {
	if args.FilePath == "" {
		return tool.Error("file_path is required for 'remove_file'."), nil
	}

	// Require approval for destructive file removal
	if tc.Approval != nil {
		approved, reason := tc.Approval(ctx, "skill_manage", args, fmt.Sprintf("Remove file '%s' from skill '%s'", args.FilePath, args.Name))
		if !approved {
			return tool.Error("Tool execution rejected: " + reason), nil
		}
	}

	tc.Emit(fmt.Sprintf("Removing file '%s' from skill '%s'...", args.FilePath, args.Name))

	if err := skillSvc.RemoveFile(ctx, wsID, slugifyForLookup(args.Name), args.FilePath); err != nil {
		return tool.Error("Failed to remove file: " + err.Error()), nil
	}

	return tool.Result(map[string]any{
		"success": true,
		"message": fmt.Sprintf("File '%s' removed from skill '%s'.", args.FilePath, args.Name),
	}), nil
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

func skillManageSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"action": {
				"type": "string",
				"enum": ["create", "patch", "edit", "delete", "write_file", "remove_file"],
				"description": "The action to perform."
			},
			"name": {
				"type": "string",
				"description": "Skill name (lowercase, hyphens/underscores, max 64 chars). Must match an existing skill for patch/edit/delete/write_file/remove_file."
			},
			"content": {
				"type": "string",
				"description": "Full SKILL.md content (YAML frontmatter + markdown body). Required for 'create' and 'edit'. For 'edit', read the skill first with skill_view() and provide the complete updated text."
			},
			"old_string": {
				"type": "string",
				"description": "Text to find in the file (required for 'patch'). Must be unique unless replace_all=true. Include enough surrounding context to ensure uniqueness."
			},
			"new_string": {
				"type": "string",
				"description": "Replacement text (required for 'patch'). Can be empty string to delete the matched text."
			},
			"replace_all": {
				"type": "boolean",
				"description": "For 'patch': replace all occurrences instead of requiring a unique match (default: false)."
			},
			"category": {
				"type": "string",
				"description": "Optional category/domain for organizing the skill (e.g., 'devops', 'data-science'). Only used with 'create'."
			},
			"file_path": {
				"type": "string",
				"description": "Path to a supporting file within the skill directory. For 'write_file'/'remove_file': required, must be under references/, templates/, scripts/, or assets/. For 'patch': optional, defaults to SKILL.md if omitted."
			},
			"file_content": {
				"type": "string",
				"description": "Content for the file. Required for 'write_file'."
			},
			"absorbed_into": {
				"type": "string",
				"description": "For 'delete' only — declares intent. Pass the umbrella skill name when this skill's content was merged into another, or an empty string when truly pruning with no forwarding target."
			}
		},
		"required": ["action", "name"]
	}`))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// splitFrontmatter extracts YAML frontmatter (delimited by ---) from markdown content.
// Returns frontmatter raw string, body string, and any error.
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	endIdx := strings.Index(content[3:], "\n---")
	if endIdx == -1 {
		return "", "", fmt.Errorf("frontmatter not closed")
	}

	frontmatter := strings.TrimSpace(content[3 : 3+endIdx])
	body := strings.TrimSpace(content[3+endIdx+4:])
	return frontmatter, body, nil
}

// slugifyForLookup converts a skill name to its slug form for database lookup.
func slugifyForLookup(name string) string {
	return slug.Make(name)
}
