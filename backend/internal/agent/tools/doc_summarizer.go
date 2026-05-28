package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/odysseythink/pantheon/core"
	"github.com/odysseythink/pantheon/tool"
)

func NewDocSummarizerSkill(tc *ToolContext) *tool.Entry {
	return &tool.Entry{
		Name:           "document-summarizer",
		Toolset:        "document",
		Description:    "List documents in this workspace or summarize a specific document by filename.",
		MaxResultChars: 12 * 1024,
		CheckFn:        func() bool { return tc.DocSvc != nil && tc.LM != nil },
		Schema: core.ToolDefinition{
			Name:        "document-summarizer",
			Description: "List or summarize workspace documents",
			Parameters:  docSummarizerSchema(),
		},
		Handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var args struct {
				Action           string `json:"action"`
				DocumentFilename string `json:"document_filename,omitempty"`
			}
			if err := json.Unmarshal(raw, &args); err != nil {
				return "", err
			}
			switch args.Action {
			case "list":
				return docSummarizerList(ctx, tc)
			case "summarize":
				if args.DocumentFilename == "" {
					return tool.Error("document_filename required"), nil
				}
				return docSummarizerSummarize(ctx, tc, args.DocumentFilename)
			default:
				return tool.Error("unknown action: " + args.Action), nil
			}
		},
	}
}

func docSummarizerList(ctx context.Context, tc *ToolContext) (string, error) {
	tc.Emit("Listing workspace documents")
	docs, err := tc.DocSvc.ListDocuments(ctx, "")
	if err != nil {
		return tool.Error("list failed: " + err.Error()), nil
	}
	out := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		preview := ""
		if d.Metadata != nil {
			preview = truncate(*d.Metadata, 200)
		}
		out = append(out, map[string]any{
			"filename": d.Filename,
			"docId":    d.DocId,
			"preview":  preview,
		})
	}
	b, _ := json.Marshal(map[string]any{"documents": out})
	return string(b), nil
}

func docSummarizerSummarize(ctx context.Context, tc *ToolContext, filename string) (string, error) {
	tc.Emit("Summarizing " + filename)
	docs, err := tc.DocSvc.ListDocuments(ctx, "")
	if err != nil {
		return tool.Error("list failed: " + err.Error()), nil
	}
	var target *struct {
		Path string
	}
	for _, d := range docs {
		if d.Filename == filename {
			target = &struct{ Path string }{Path: d.Docpath}
			break
		}
	}
	if target == nil {
		return tool.Error(fmt.Sprintf("document %q not found", filename)), nil
	}

	content, err := os.ReadFile(target.Path)
	if err != nil {
		return tool.Error("read document: " + err.Error()), nil
	}
	if len(content) > 64*1024 {
		content = content[:64*1024]
	}

	resp, err := tc.LM.Generate(ctx, &core.Request{
		SystemPrompt: "You are a concise summarizer. Output 5-10 bullet points.",
		Messages: []core.Message{
			core.NewTextMessage(core.MESSAGE_ROLE_USER, "Summarize:\n"+string(content)),
		},
	})
	if err != nil {
		return tool.Error("summarize call: " + err.Error()), nil
	}
	return resp.Message.Text(), nil
}

func docSummarizerSchema() *core.Schema {
	return core.MustSchemaFromJSON([]byte(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["list", "summarize"]},
			"document_filename": {"type": "string"}
		},
		"required": ["action"]
	}`))
}
