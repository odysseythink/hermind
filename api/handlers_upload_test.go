package api

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
)

func TestUpload(t *testing.T) {
	root := t.TempDir()
	srv, err := NewServer(&ServerOpts{
		Config:       &config.Config{},
		InstanceRoot: root,
		Version:      "test",
		Streams:      NewMemoryStreamHub(),
	})
	require.NoError(t, err)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.txt")
	require.NoError(t, err)
	_, err = part.Write([]byte("hello world"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	srv.Router().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "test.txt")

	attDir := filepath.Join(root, "attachments")
	entries, err := os.ReadDir(attDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
