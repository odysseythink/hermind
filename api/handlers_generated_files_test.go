package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/odysseythink/hermind/config"
)

func newTestServerWithGeneratedFiles(t *testing.T) (*Server, string) {
	t.Helper()
	tmpDir := t.TempDir()
	srv, err := NewServer(&ServerOpts{
		Config:       &config.Config{},
		InstanceRoot: tmpDir,
	})
	require.NoError(t, err)
	return srv, filepath.Join(tmpDir, "generated-files")
}

func TestGeneratedFileDownload_Success(t *testing.T) {
	srv, tmpDir := newTestServerWithGeneratedFiles(t)

	// Create a test file
	err := os.MkdirAll(tmpDir, 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "text-12345678-1234-1234-1234-123456789abc.txt"), []byte("hello"), 0o644)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/generated-files/text-12345678-1234-1234-1234-123456789abc.txt", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "hello", rr.Body.String())
	require.Equal(t, "text/plain; charset=utf-8", rr.Header().Get("Content-Type"))
}

func TestGeneratedFileDownload_InvalidFilename(t *testing.T) {
	srv, _ := newTestServerWithGeneratedFiles(t)

	req := httptest.NewRequest(http.MethodGet, "/api/generated-files/foo..bar.txt", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGeneratedFileDownload_NotFound(t *testing.T) {
	srv, _ := newTestServerWithGeneratedFiles(t)

	req := httptest.NewRequest(http.MethodGet, "/api/generated-files/text-12345678-1234-1234-1234-123456789abc.txt", nil)
	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}
