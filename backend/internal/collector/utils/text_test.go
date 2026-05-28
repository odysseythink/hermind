package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTextType_True(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world\nthis is a text file."), 0644))

	assert.True(t, IsTextType(filePath))
}

func TestIsTextType_False(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "binary.bin")
	// Write mostly null bytes and control chars (>10% threshold)
	data := make([]byte, 100)
	for i := range data {
		data[i] = 0x00
	}
	require.NoError(t, os.WriteFile(filePath, data, 0644))

	assert.False(t, IsTextType(filePath))
}

func TestIsTextType_EdgeCase(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "edge.txt")
	// Exactly 9% control chars (< 10% threshold)
	data := make([]byte, 100)
	for i := 0; i < 9; i++ {
		data[i] = 0x01
	}
	for i := 9; i < 100; i++ {
		data[i] = 'a'
	}
	require.NoError(t, os.WriteFile(filePath, data, 0644))

	assert.True(t, IsTextType(filePath))
}
