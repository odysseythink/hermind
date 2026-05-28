package processors

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/odysseythink/hermind/backend/internal/collector/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMboxExtractor_Supports(t *testing.T) {
	e := NewMboxExtractor()
	assert.True(t, e.Supports(".mbox"))
	assert.False(t, e.Supports(".txt"))
}

func TestMboxExtractor_Extract(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.mbox")
	data := `From user@example.com Mon Jan 01 00:00:00 2024
From: user@example.com
To: recipient@example.com
Subject: Test Message One

This is the body of message one.
It has multiple lines.

From user@example.com Mon Jan 02 00:00:00 2024
From: another@example.com
To: recipient@example.com
Subject: Test Message Two

This is the body of message two.
`
	require.NoError(t, os.WriteFile(filePath, []byte(data), 0644))

	e := NewMboxExtractor()
	out, err := e.Extract(context.Background(), pipeline.ExtractInput{FilePath: filePath})
	require.NoError(t, err)
	assert.Contains(t, out.Content, "This is the body of message one.")
	assert.Contains(t, out.Content, "This is the body of message two.")
	assert.NotContains(t, out.Content, "Subject:")
}
