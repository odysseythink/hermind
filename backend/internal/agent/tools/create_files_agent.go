package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func NewCreateFilesAgentSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "create-files-agent",
		Toolset:        "create-files",
		Description:    "Generate a file (txt, md, docx, pdf, or xlsx) and save it to the workspace's generated-files folder. Returns the saved path.",
		Emoji:          "📝",
		MaxResultChars: 2 * 1024,
		CheckFn: func() bool {
			return tc.Cfg != nil && tc.Cfg.AgentCreateFilesEnabled
		},
		Schema: core.ToolDefinition{
			Name:        "create-files-agent",
			Description: "Create txt/md/docx/pdf/xlsx files",
			Parameters:  createFilesSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Format   string `json:"format"`
				Filename string `json:"filename"`
				Content  any    `json:"content"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return tool.Error(err.Error()), nil
			}
			if tc.Cfg == nil {
				return tool.Error("create-files not configured"), nil
			}

			switch args.Format {
			case "txt", "md", "docx", "pdf", "pptx", "xlsx":
				// ok
			default:
				return tool.Error("unknown format: " + args.Format), nil
			}

			// Approval gate
			if tc.Approval != nil {
				desc := fmt.Sprintf("Create %s file: %s", args.Format, args.Filename)
				if approved, reason := tc.Approval(ctx, "create-files-agent:"+args.Format, args, desc); !approved {
					return tool.Error("rejected: " + reason), nil
				}
			}

			base := sanitiseFilename(args.Filename)
			if base == "" {
				base = "untitled"
			}
			uniqueName := fmt.Sprintf("%s-%s.%s", time.Now().Format("20060102-150405"), uuid.NewString()[:8], args.Format)
			if base != "" && base != "untitled" {
				uniqueName = base + "-" + uniqueName
			}
			dst := filepath.Join(tc.Cfg.AgentCreateFilesDir, uniqueName)

			contentStr, ok := args.Content.(string)
			if !ok {
				return tool.Error("content must be a string for " + args.Format + " format"), nil
			}

			switch args.Format {
			case "txt", "md":
				if err := os.WriteFile(dst, []byte(contentStr), 0o644); err != nil {
					return tool.Error(err.Error()), nil
				}
			case "docx":
				if err := writeDocxFile(ctx, dst, contentStr, args.Filename); err != nil {
					return tool.Error(err.Error()), nil
				}
			case "pdf":
				if err := writePDFFile(ctx, dst, contentStr, args.Filename); err != nil {
					return tool.Error(err.Error()), nil
				}
			case "xlsx":
				if err := writeXLSXFile(ctx, dst, args.Content); err != nil {
					return tool.Error(err.Error()), nil
				}
			case "pptx":
				if err := writePptxFile(ctx, dst, contentStr, args.Filename); err != nil {
					return tool.Error(err.Error()), nil
				}
			}

			tc.Emit("Created " + uniqueName)
			return tool.Result(map[string]any{
				"saved_path": dst,
				"filename":   uniqueName,
				"format":     args.Format,
			}), nil
		},
	}
}

func createFilesSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"format":   {"type": "string", "enum": ["txt","md","docx","pdf","pptx","xlsx"]},
			"filename": {"type": "string"},
			"content":  {
				"oneOf": [
					{"type": "string", "description": "Text content for txt/md/docx/pdf"},
					{
						"type": "object",
						"description": "Sheet data for xlsx",
						"properties": {
							"sheets": {
								"type": "array",
								"items": {
									"type": "object",
									"properties": {
										"name": {"type": "string"},
										"rows": {
											"type": "array",
											"items": {"type": "array", "items": {"type": "string"}}
										}
									}
								}
							}
						}
					}
				]
			}
		},
		"required": ["format", "filename", "content"]
	}`))
}

// sanitiseFilename strips path separators and parent-traversal sequences.
func sanitiseFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}
