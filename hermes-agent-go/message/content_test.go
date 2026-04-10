package message

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextContentAccessors(t *testing.T) {
	c := TextContent("hello")
	assert.True(t, c.IsText())
	assert.Equal(t, "hello", c.Text())
	assert.Nil(t, c.Blocks())
}

func TestBlockContentAccessors(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "hello"},
		{Type: "image_url", ImageURL: &Image{URL: "http://x.png"}},
	}
	c := BlockContent(blocks)
	assert.False(t, c.IsText())
	assert.Equal(t, "", c.Text())
	assert.Len(t, c.Blocks(), 2)
}

func TestContentMarshalJSONText(t *testing.T) {
	c := TextContent("hello")
	data, err := c.MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, `"hello"`, string(data))
}

func TestContentMarshalJSONBlocks(t *testing.T) {
	c := BlockContent([]ContentBlock{{Type: "text", Text: "hi"}})
	data, err := c.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"text","text":"hi"}]`, string(data))
}

func TestContentUnmarshalJSONAcceptsString(t *testing.T) {
	var c Content
	err := json.Unmarshal([]byte(`"hello"`), &c)
	require.NoError(t, err)
	assert.True(t, c.IsText())
	assert.Equal(t, "hello", c.Text())
}

func TestContentUnmarshalJSONAcceptsArray(t *testing.T) {
	var c Content
	err := json.Unmarshal([]byte(`[{"type":"text","text":"hi"}]`), &c)
	require.NoError(t, err)
	assert.False(t, c.IsText())
	require.Len(t, c.Blocks(), 1)
	assert.Equal(t, "hi", c.Blocks()[0].Text)
}

func TestContentUnmarshalJSONRejectsInvalid(t *testing.T) {
	var c Content
	err := json.Unmarshal([]byte(`123`), &c)
	assert.Error(t, err)
}
