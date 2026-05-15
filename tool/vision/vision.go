// Package vision provides an image analysis tool backed by a
// vision-capable LLM. The handler constructs a multimodal message with
// the user's prompt and an image_url block, sends it via the provided
// Provider, and returns the assistant's text reply.
package vision

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/message"
	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/pantheon/core"
)

const visionSchema = `{
  "type":"object",
  "properties":{
    "image_url":{"type":"string","description":"Public URL of the image to analyze"},
    "prompt":{"type":"string","description":"Question or instruction for the image (default: describe)"},
    "detail":{"type":"string","enum":["low","high","auto"],"description":"Image detail level (OpenAI-style)"}
  },
  "required":["image_url"]
}`

// Args is the JSON shape vision_analyze accepts.
type Args struct {
	ImageURL string `json:"image_url"`
	Prompt   string `json:"prompt,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Model    string `json:"model,omitempty"`
}

// Result is the JSON shape vision_analyze returns.
type Result struct {
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
}

// newHandler builds the tool handler that calls prov.Generate with a
// multimodal request. defaultModel is the model name used when the
// caller doesn't specify one.
func newHandler(prov core.LanguageModel, defaultModel string) tool.Handler {
	return func(ctx context.Context, raw json.RawMessage) (string, error) {
		var args Args
		if err := json.Unmarshal(raw, &args); err != nil {
			return tool.ToolError("invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(args.ImageURL) == "" {
			return tool.ToolError("image_url is required"), nil
		}
		prompt := args.Prompt
		if strings.TrimSpace(prompt) == "" {
			prompt = "Describe this image in detail."
		}
		detail := args.Detail
		if detail == "" {
			detail = "auto"
		}
		model := args.Model
		if model == "" {
			model = defaultModel
		}

		msg := message.HermindMessage{
			Role: core.MESSAGE_ROLE_USER,
			Content: []core.ContentParter{
				core.TextPart{Text: prompt},
				core.ImagePart{
					URL:    args.ImageURL,
					Detail: detail,
				},
			},
		}
		req := &core.Request{
			Messages: []core.Message{message.ToPantheon(msg)},
		}

		resp, err := prov.Generate(ctx, req)
		if err != nil {
			return tool.ToolError(fmt.Sprintf("vision call: %v", err)), nil
		}
		if resp == nil {
			return tool.ToolError("vision: empty response"), nil
		}
		text := ""
		for _, part := range resp.Message.Content {
			if p, ok := part.(core.TextPart); ok {
				text += p.Text
			}
		}
		return tool.ToolResult(Result{Description: text, Model: model}), nil
	}
}

// Register registers vision_analyze into reg with the given provider.
// If prov is nil the tool is not registered.
func Register(reg *tool.Registry, prov core.LanguageModel, defaultModel string) {
	if prov == nil {
		return
	}
	reg.Register(&tool.Entry{
		Name:        "vision_analyze",
		Toolset:     "vision",
		Description: "Analyze an image URL using a vision-capable LLM.",
		Emoji:       "👁",
		Handler:     newHandler(prov, defaultModel),
		Schema: core.ToolDefinition{
			Name:        "vision_analyze",
			Description: "Analyze an image. Supply a public image_url and an optional prompt.",
			Parameters:  core.MustSchemaFromJSON([]byte(visionSchema)),
		},
	})
}
