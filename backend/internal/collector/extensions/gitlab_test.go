package extensions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xanzy/go-gitlab"
)

func TestGitLabExtension_Name(t *testing.T) {
	g := NewGitLabExtension()
	assert.Equal(t, "gitlab", g.Name())
}

func TestGitLabExtension_Handle_UnsupportedMethod(t *testing.T) {
	g := NewGitLabExtension()
	_, err := g.Handle(context.Background(), "/ext/gitlab-repo", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestGitLabExtension_Handle_UnknownEndpoint(t *testing.T) {
	g := NewGitLabExtension()
	_, err := g.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestGitLabExtension_loadRepo(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/tree", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]gitlab.TreeNode{
			{Name: "README.md", Type: "blob", Path: "README.md"},
			{Name: "main.go", Type: "blob", Path: "src/main.go"},
			{Name: "ignored.exe", Type: "blob", Path: "ignored.exe"},
		})
	})

	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/files/", func(w http.ResponseWriter, r *http.Request) {
		filePath := strings.TrimPrefix(r.URL.Path, "/api/v4/projects/owner%2Frepo/repository/files/")
		filePath = strings.TrimSuffix(filePath, "/raw")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gitlab.File{
			FileName: filePath,
			Content:  "ZmlsZSBjb250ZW50", // base64("file content")
			Encoding: "base64",
		})
	})

	g := NewGitLabExtensionWithClient(server.URL+"/api/v4", server.Client())

	body, _ := json.Marshal(RepoRequest{RepoURL: "https://gitlab.com/owner/repo"})
	resp, err := g.loadRepo(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	files, ok := resp.Data["files"].([]FileInfo)
	require.True(t, ok)
	assert.Len(t, files, 2) // README.md and main.go; ignored.exe skipped
}

func TestGitLabExtension_getBranches(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/branches", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]gitlab.Branch{
			{Name: "main"},
			{Name: "develop"},
		})
	})

	g := NewGitLabExtensionWithClient(server.URL+"/api/v4", server.Client())

	body, _ := json.Marshal(RepoRequest{RepoURL: "https://gitlab.com/owner/repo"})
	resp, err := g.getBranches(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	branches, ok := resp.Data["branches"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"main", "develop"}, branches)
}

func TestParseGitLabProjectPath(t *testing.T) {
	p, err := parseGitLabProjectPath("https://gitlab.com/owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", p)

	p, err = parseGitLabProjectPath("https://gitlab.com/owner/repo.git")
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", p)

	_, err = parseGitLabProjectPath("not-a-url")
	assert.Error(t, err)
}

func TestGitLabExtension_loadRepo_InvalidBody(t *testing.T) {
	g := NewGitLabExtension()
	_, err := g.loadRepo(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestGitLabExtension_getBranches_InvalidBody(t *testing.T) {
	g := NewGitLabExtension()
	_, err := g.getBranches(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestGitLabExtension_Handle(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/tree", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]gitlab.TreeNode{
			{Name: "README.md", Type: "blob", Path: "README.md"},
		})
	})
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/files/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gitlab.File{
			FileName: "README.md",
			Content:  "cmVhZG1l",
			Encoding: "base64",
		})
	})
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/branches", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]gitlab.Branch{
			{Name: "main"},
		})
	})

	g := NewGitLabExtensionWithClient(server.URL+"/api/v4", server.Client())

	body, _ := json.Marshal(RepoRequest{RepoURL: "https://gitlab.com/owner/repo"})
	resp, err := g.Handle(context.Background(), "/ext/gitlab-repo", "POST", body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	resp, err = g.Handle(context.Background(), "/ext/gitlab-repo/branches", "POST", body)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	branches, _ := resp.Data["branches"].([]string)
	assert.Equal(t, []string{"main"}, branches)
}

func TestGitLabExtension_loadRepo_TreeError(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/tree", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "404 Not Found"})
	})

	g := NewGitLabExtensionWithClient(server.URL+"/api/v4", server.Client())

	body, _ := json.Marshal(RepoRequest{RepoURL: "https://gitlab.com/owner/repo"})
	_, err := g.loadRepo(context.Background(), body)
	assert.Error(t, err)
}

func TestGitLabExtension_downloadFile_Error(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/api/v4/projects/owner%2Frepo/repository/files/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "404 Not Found"})
	})

	g := NewGitLabExtensionWithClient(server.URL+"/api/v4", server.Client())
	_, err := g.downloadFile(context.Background(), g.client, "owner/repo", "README.md", "main")
	assert.Error(t, err)
}
