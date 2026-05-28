package integration

import (
	"strings"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/chunker"
	"github.com/stretchr/testify/assert"
)

func TestChunkerSplitShortText(t *testing.T) {
	c := chunker.NewChunker(100, 10, "")
	chunks := c.Split("Hello world.")
	assert.Len(t, chunks, 1)
	assert.Equal(t, "Hello world.", chunks[0])
}

func TestChunkerSplitLongText(t *testing.T) {
	text := strings.Repeat("This is a sentence. ", 50)
	c := chunker.NewChunker(100, 10, "")
	chunks := c.Split(text)
	assert.GreaterOrEqual(t, len(chunks), 2)
	for _, ch := range chunks {
		assert.LessOrEqual(t, len(ch), 100)
	}
}

func TestChunkerPrefix(t *testing.T) {
	c := chunker.NewChunker(100, 10, "prefix: ")
	chunks := c.Split("Hello world. This is another sentence.")
	for _, ch := range chunks {
		assert.True(t, strings.HasPrefix(ch, "prefix: "))
	}
}

func TestChunkerParagraphBoundaries(t *testing.T) {
	text := "Paragraph one.\n\nParagraph two.\n\nParagraph three."
	c := chunker.NewChunker(200, 10, "")
	chunks := c.Split(text)
	assert.GreaterOrEqual(t, len(chunks), 1)
}
