package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObsidianExtension_Name(t *testing.T) {
	o := NewObsidianExtension()
	assert.Equal(t, "obsidian", o.Name())
}

func TestObsidianExtension_Handle_UnsupportedMethod(t *testing.T) {
	o := NewObsidianExtension()
	_, err := o.Handle(context.Background(), "/ext/obsidian/vault", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestObsidianExtension_Handle_UnknownEndpoint(t *testing.T) {
	o := NewObsidianExtension()
	_, err := o.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestObsidianExtension_loadVault(t *testing.T) {
	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "note1.md"), []byte("# Note 1\n\nContent here."), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "note2.md"), []byte("---\n{\"title\": \"Note 2\", \"tags\": [\"test\"]}\n---\n\nContent two."), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("readme"), 0644)

	o := NewObsidianExtension()
	body, _ := json.Marshal(ObsidianRequest{VaultPath: tmpDir})
	resp, err := o.loadVault(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	files, ok := resp.Data["files"].([]ObsidianFile)
	require.True(t, ok)
	assert.Len(t, files, 3)

	var foundNote2 bool
	for _, f := range files {
		if f.Name == "note2.md" {
			foundNote2 = true
			assert.Equal(t, "Note 2", f.FrontMatter["title"])
			assert.Equal(t, "Content two.", f.Content)
		}
	}
	assert.True(t, foundNote2)
}

func TestObsidianExtension_loadVault_InvalidBody(t *testing.T) {
	o := NewObsidianExtension()
	_, err := o.loadVault(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestObsidianExtension_loadVault_MissingPath(t *testing.T) {
	o := NewObsidianExtension()
	body, _ := json.Marshal(ObsidianRequest{})
	_, err := o.loadVault(context.Background(), body)
	assert.Error(t, err)
}

func TestExtractFrontMatter(t *testing.T) {
	fm, body := extractFrontMatter("---\n{\"title\": \"Test\"}\n---\n\nBody content")
	require.NotNil(t, fm)
	assert.Equal(t, "Test", fm["title"])
	assert.Equal(t, "Body content", body)

	fm, body = extractFrontMatter("No frontmatter here")
	assert.Nil(t, fm)
	assert.Equal(t, "No frontmatter here", body)

	fm, body = extractFrontMatter("---\nnot valid json\n---\n\nBody")
	assert.Nil(t, fm)
	assert.Equal(t, "---\nnot valid json\n---\n\nBody", body)
}
