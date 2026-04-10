package message

import (
	"encoding/json"
	"fmt"
)

// Content is a typed union of plain text and structured content blocks.
// Exactly one of text or blocks is populated. Never both.
// Use TextContent(s) or BlockContent(b) to construct.
type Content struct {
	text   string
	blocks []ContentBlock
}

// TextContent creates a Content holding a plain text string.
func TextContent(s string) Content {
	return Content{text: s}
}

// BlockContent creates a Content holding a list of structured blocks.
func BlockContent(blocks []ContentBlock) Content {
	return Content{blocks: blocks}
}

// IsText reports whether the Content is the plain-text form.
func (c Content) IsText() bool { return c.blocks == nil }

// Text returns the plain-text form. Empty string if IsText() is false.
func (c Content) Text() string { return c.text }

// Blocks returns the structured-blocks form. Nil if IsText() is true.
func (c Content) Blocks() []ContentBlock { return c.blocks }

// MarshalJSON produces the OpenAI-compatible shape: string OR array.
func (c Content) MarshalJSON() ([]byte, error) {
	if c.IsText() {
		return json.Marshal(c.text)
	}
	return json.Marshal(c.blocks)
}

// UnmarshalJSON accepts either a string or an array of ContentBlock.
func (c *Content) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("message: empty content")
	}
	// Try string first (cheapest discriminator)
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("message: invalid string content: %w", err)
		}
		c.text = s
		c.blocks = nil
		return nil
	}
	// Fall back to array
	if data[0] == '[' {
		var blocks []ContentBlock
		if err := json.Unmarshal(data, &blocks); err != nil {
			return fmt.Errorf("message: invalid block content: %w", err)
		}
		c.text = ""
		c.blocks = blocks
		return nil
	}
	return fmt.Errorf("message: content must be string or array, got %q", data[:1])
}

// ContentBlock is one element of a structured content array.
// Used for multimodal content (images) and tool results.
type ContentBlock struct {
	Type     string `json:"type"` // "text", "image_url", "tool_use", "tool_result"
	Text     string `json:"text,omitempty"`
	ImageURL *Image `json:"image_url,omitempty"`
}

// Image represents an image reference in a content block.
type Image struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}
