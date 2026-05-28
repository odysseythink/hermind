package collector

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalCollector(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, client)
	client.Close()
}

func TestClient_Online(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	assert.True(t, client.Online(context.Background()))
}

func TestClient_AcceptedFileTypes(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	types, err := client.AcceptedFileTypes(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, types)
}

func TestClient_ProcessDocument(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	filePath := filepath.Join(tmpDir, "hello.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world from document"), 0644))

	resp, err := client.ProcessDocument(context.Background(), filePath, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.Len(t, resp.Documents, 1)
	assert.Equal(t, "hello world from document", resp.Documents[0].PageContent)
}

func TestClient_ParseDocument(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	filePath := filepath.Join(tmpDir, "parse-me.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("parse this text"), 0644))

	resp, err := client.ParseDocument(context.Background(), "parse-me.txt", ParseOptions{AbsolutePath: filePath})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.Len(t, resp.Documents, 1)
	assert.True(t, resp.Documents[0].IsDirectUpload)
	assert.Equal(t, "parse this text", resp.Documents[0].PageContent)
}

func TestClient_ProcessRawText(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	resp, err := client.ProcessRawText(context.Background(), "raw text content", map[string]string{"title": "My Raw Text"})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.Len(t, resp.Documents, 1)
	assert.Equal(t, "raw text content", resp.Documents[0].PageContent)
	assert.Equal(t, "My Raw Text", resp.Documents[0].Title)
}

func TestClient_ProcessLink(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body><p>Hello from test server</p></body></html>`))
	}))
	defer server.Close()

	resp, err := client.ProcessLink(context.Background(), server.URL, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	require.Len(t, resp.Documents, 1)
	assert.Contains(t, resp.Documents[0].PageContent, "Hello from test server")
}

func TestClient_GetLinkContent(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body><p>Link content here</p></body></html>`))
	}))
	defer server.Close()

	resp, err := client.GetLinkContent(context.Background(), server.URL, "text")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.Success)
	assert.Contains(t, resp.Content, "Link content here")
}

func TestClient_ForwardExtensionRequest(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	// Unknown endpoint should return an error.
	_, err = client.ForwardExtensionRequest(context.Background(), "/ext/unknown", "POST", "{}")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Known endpoint with invalid body should return an error from the extension.
	_, err = client.ForwardExtensionRequest(context.Background(), "/ext/youtube-transcript", "POST", "invalid-json")
	require.Error(t, err)
}

func TestClient_ProcessDocument_UnsupportedType(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	defer client.Close()

	// Write a binary file with null bytes so it is not detected as text.
	filePath := filepath.Join(tmpDir, "unknown.xyz")
	require.NoError(t, os.WriteFile(filePath, []byte{0x00, 0x01, 0x02, 0x03}, 0644))

	resp, err := client.ProcessDocument(context.Background(), filePath, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.False(t, resp.Success)
}

func TestClient_Close(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewLocalCollector(tmpDir)
	require.NoError(t, err)
	// Close should not panic.
	client.Close()
}

func TestClient_NewClientAlias(t *testing.T) {
	tmpDir := t.TempDir()
	client, err := NewClient(tmpDir)
	require.NoError(t, err)
	assert.NotNil(t, client)
	client.Close()
}
