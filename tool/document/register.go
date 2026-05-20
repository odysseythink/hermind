package document

import (
	"path/filepath"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

// RegisterAll registers all document creation tools.
func RegisterAll(reg *tool.Registry, instanceRoot string) {
	outputDir := filepath.Join(instanceRoot, "generated-files")

	RegisterCreateTextFile(reg, outputDir)
	RegisterCreateExcel(reg, outputDir)
	RegisterCreatePDF(reg, outputDir)

	// Word and PowerPoint via Node.js subprocess
	scriptDir := filepath.Join(instanceRoot, "..", "document-scripts")
	wrapper := NewNodeJSWrapper(scriptDir, outputDir)

	reg.Register(&tool.Entry{
		Name:        "create_word_document",
		Toolset:     "document_creation",
		Description: "Create a Microsoft Word document (.docx) from markdown or plain text content.",
		Emoji:       "📄",
		Handler:     NewCreateWordHandler(wrapper),
		CheckFn:     wrapper.IsAvailable,
		Schema: core.ToolDefinition{
			Name:        "create_word_document",
			Description: "Create a Microsoft Word document (.docx) from markdown or plain text content. Supports headings, tables, lists, and styling themes.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
				"type": "object",
				"properties": {
					"filename": {"type": "string", "description": "The filename for the Word document. Will add .docx if not present."},
					"title": {"type": "string", "description": "Document title for metadata and title page."},
					"content": {"type": "string", "description": "The content to convert to a Word document. Supports markdown formatting."},
					"theme": {"type": "string", "enum": ["neutral", "blue", "warm"], "default": "neutral"},
					"margins": {"type": "string", "enum": ["normal", "narrow", "wide"], "default": "normal"},
					"includeTitlePage": {"type": "boolean", "default": false}
				},
				"required": ["filename", "content"]
			}`)),
		},
	})

	reg.Register(&tool.Entry{
		Name:        "create_pptx_presentation",
		Toolset:     "document_creation",
		Description: "Create a PowerPoint presentation (.pptx) with slides, themes, and bullet points.",
		Emoji:       "📊",
		Handler:     NewCreatePPTXHandler(wrapper),
		CheckFn:     wrapper.IsAvailable,
		Schema: core.ToolDefinition{
			Name:        "create_pptx_presentation",
			Description: "Create a PowerPoint presentation (.pptx). Provide a title, theme, and section outlines with key points. Each section becomes a slide.",
			Parameters: core.MustSchemaFromJSON([]byte(`{
				"type": "object",
				"properties": {
					"filename": {"type": "string", "description": "The filename for the presentation. Will add .pptx if not present."},
					"title": {"type": "string", "description": "Presentation title."},
					"theme": {"type": "string", "enum": ["corporate", "dark", "light"], "default": "corporate"},
					"sections": {
						"type": "array",
						"items": {
							"type": "object",
							"properties": {
								"title": {"type": "string"},
								"keyPoints": {"type": "array", "items": {"type": "string"}},
								"instructions": {"type": "string"}
							},
							"required": ["title", "keyPoints"]
						}
					}
				},
				"required": ["filename", "title", "sections"]
			}`)),
		},
	})
}
