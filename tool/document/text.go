package document

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// NewCreateTextFileHandler returns a handler for create_text_file.
func NewCreateTextFileHandler(outputDir string) tool.Handler {
	mgr := NewManager(outputDir)
	return func(_ context.Context, args json.RawMessage) (string, error) {
		var params struct {
			Filename  string `json:"filename"`
			Extension string `json:"extension"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return tool.ToolError(fmt.Sprintf("invalid args: %v", err)), nil
		}
		if params.Filename == "" {
			params.Filename = "document"
		}
		if params.Extension == "" {
			params.Extension = "txt"
		}
		ext := strings.ToLower(strings.TrimPrefix(params.Extension, "."))
		if !strings.Contains(params.Filename, ".") {
			params.Filename = params.Filename + "." + ext
		}
		finalExt := params.Filename[strings.LastIndex(params.Filename, ".")+1:]

		buf := []byte(params.Content)
		saved, err := mgr.Save("text", finalExt, buf, params.Filename)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("save file: %v", err)), nil
		}

		return resultJSON(saved, fmt.Sprintf("Successfully created text file '%s' (%d bytes).", saved.DisplayFilename, saved.FileSize)), nil
	}
}

func resultJSON(saved *SavedFile, message string) string {
	b, _ := json.Marshal(map[string]interface{}{
		"displayFilename": saved.DisplayFilename,
		"storageFilename": saved.Filename,
		"fileSize":        saved.FileSize,
		"downloadUrl":     "/api/generated-files/" + saved.Filename,
		"message":         message,
	})
	return string(b)
}

// CreateTextFileSchema is the JSON schema for create_text_file.
const CreateTextFileSchema = `{
	"type": "object",
	"properties": {
		"filename": {"type": "string", "description": "The filename for the text file. If no extension is provided, the extension parameter will be used."},
		"extension": {"type": "string", "description": "The file extension to use (without the dot). Defaults to 'txt'. Common options: txt, md, json, csv, html, xml, yaml, log.", "default": "txt"},
		"content": {"type": "string", "description": "The text content to write to the file."}
	},
	"required": ["filename", "content"]
}`

// RegisterCreateTextFile registers the create_text_file tool.
func RegisterCreateTextFile(reg *tool.Registry, outputDir string) {
	reg.Register(&tool.Entry{
		Name:        "create_text_file",
		Toolset:     "document_creation",
		Description: "Create a text file with arbitrary content. Supports .txt, .md, .json, .csv, .html, .xml, .yaml, .log, and more.",
		Emoji:       "📝",
		Handler:     NewCreateTextFileHandler(outputDir),
		Schema: core.ToolDefinition{
			Name:        "create_text_file",
			Description: "Create a text file with arbitrary content. Provide the content and an optional file extension (defaults to .txt). Common extensions include .txt, .md, .json, .csv, .html, .xml, .yaml, .log, etc.",
			Parameters:  core.MustSchemaFromJSON([]byte(CreateTextFileSchema)),
		},
	})
}
