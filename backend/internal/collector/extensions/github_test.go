package extensions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"

	"github.com/google/go-github/v63/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubExtension_Name(t *testing.T) {
	g := NewGitHubExtension()
	assert.Equal(t, "github", g.Name())
}

func TestGitHubExtension_Handle_UnsupportedMethod(t *testing.T) {
	g := NewGitHubExtension()
	_, err := g.Handle(context.Background(), "/ext/github-repo", "GET", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestGitHubExtension_Handle_UnknownEndpoint(t *testing.T) {
	g := NewGitHubExtension()
	_, err := g.Handle(context.Background(), "/ext/unknown", "POST", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown endpoint")
}

func TestGitHubExtension_loadRepo(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	// Contents endpoint
	mux.HandleFunc("/repos/owner/repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		pathPart := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo/contents/")
		w.Header().Set("Content-Type", "application/json")
		if pathPart == "" {
			json.NewEncoder(w).Encode([]github.RepositoryContent{
				{Name: github.String("README.md"), Type: github.String("file"), Path: github.String("README.md"), DownloadURL: github.String(server.URL + "/raw/README.md")},
				{Name: github.String("src"), Type: github.String("dir"), Path: github.String("src")},
			})
			return
		}
		if pathPart == "src" {
			json.NewEncoder(w).Encode([]github.RepositoryContent{
				{Name: github.String("main.go"), Type: github.String("file"), Path: github.String("src/main.go"), DownloadURL: github.String(server.URL + "/raw/src/main.go")},
			})
			return
		}
		// Individual file content requests
		if pathPart == "README.md" || pathPart == "src/main.go" {
			json.NewEncoder(w).Encode(github.RepositoryContent{
				Name:     github.String(path.Base(pathPart)),
				Type:     github.String("file"),
				Path:     github.String(pathPart),
				Encoding: github.String("base64"),
				Content:  github.String("ZmlsZSBjb250ZW50"), // base64("file content")
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// Raw content endpoint
	mux.HandleFunc("/raw/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("file content"))
	})

	client := github.NewClient(nil)
	url, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = url

	g := NewGitHubExtensionWithClient(client.Client())
	g.client = client

	body, _ := json.Marshal(RepoRequest{RepoURL: "https://github.com/owner/repo"})
	resp, err := g.loadRepo(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	files, ok := resp.Data["files"].([]FileInfo)
	require.True(t, ok)
	assert.Len(t, files, 2)
}

func TestGitHubExtension_getBranches(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/repos/owner/repo/branches", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]github.Branch{
			{Name: github.String("main")},
			{Name: github.String("develop")},
		})
	})

	client := github.NewClient(nil)
	url, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = url

	g := NewGitHubExtensionWithClient(client.Client())
	g.client = client

	body, _ := json.Marshal(RepoRequest{RepoURL: "https://github.com/owner/repo"})
	resp, err := g.getBranches(context.Background(), body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	branches, ok := resp.Data["branches"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"main", "develop"}, branches)
}

func TestParseGitHubURL(t *testing.T) {
	owner, repo, err := parseGitHubURL("https://github.com/owner/repo")
	require.NoError(t, err)
	assert.Equal(t, "owner", owner)
	assert.Equal(t, "repo", repo)

	_, _, err = parseGitHubURL("https://github.com/owner")
	assert.Error(t, err)

	_, _, err = parseGitHubURL("not-a-url")
	assert.Error(t, err)
}

func TestIsSupportedExt(t *testing.T) {
	assert.True(t, isSupportedExt(".go"))
	assert.True(t, isSupportedExt(".md"))
	assert.False(t, isSupportedExt(".exe"))
}

func TestGitHubExtension_loadRepo_InvalidBody(t *testing.T) {
	g := NewGitHubExtension()
	_, err := g.loadRepo(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestGitHubExtension_getBranches_InvalidBody(t *testing.T) {
	g := NewGitHubExtension()
	_, err := g.getBranches(context.Background(), []byte("invalid"))
	assert.Error(t, err)
}

func TestGitHubExtension_Handle(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/repos/owner/repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		pathPart := strings.TrimPrefix(r.URL.Path, "/repos/owner/repo/contents/")
		w.Header().Set("Content-Type", "application/json")
		if pathPart == "" {
			json.NewEncoder(w).Encode([]github.RepositoryContent{
				{Name: github.String("README.md"), Type: github.String("file"), Path: github.String("README.md"), DownloadURL: github.String(server.URL + "/raw/README.md")},
			})
			return
		}
		if pathPart == "README.md" {
			json.NewEncoder(w).Encode(github.RepositoryContent{
				Name:     github.String("README.md"),
				Type:     github.String("file"),
				Path:     github.String("README.md"),
				Encoding: github.String("base64"),
				Content:  github.String("cmVhZG1l"), // base64("readme")
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/raw/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("readme"))
	})
	mux.HandleFunc("/repos/owner/repo/branches", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]github.Branch{
			{Name: github.String("main")},
		})
	})

	client := github.NewClient(nil)
	url, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = url

	g := NewGitHubExtensionWithClient(client.Client())
	g.client = client

	// Test repo endpoint
	body, _ := json.Marshal(RepoRequest{RepoURL: "https://github.com/owner/repo"})
	resp, err := g.Handle(context.Background(), "/ext/github-repo", "POST", body)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	// Test branches endpoint
	resp, err = g.Handle(context.Background(), "/ext/github-repo/branches", "POST", body)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	branches, _ := resp.Data["branches"].([]string)
	assert.Equal(t, []string{"main"}, branches)
}

func TestGitHubExtension_downloadContent_NoDownloadURL(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/repos/owner/repo/contents/nodl", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(github.RepositoryContent{
			Name: github.String("nodl"), Type: github.String("file"), Path: github.String("nodl"),
		})
	})

	client := github.NewClient(nil)
	url, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = url

	g := NewGitHubExtensionWithClient(client.Client())
	g.client = client

	_, err := g.downloadContent(context.Background(), client, "owner", "repo", "nodl", nil)
	assert.Error(t, err)
}

func TestGitHubExtension_walkRepo_Error(t *testing.T) {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/repos/owner/repo/contents/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	})

	client := github.NewClient(nil)
	url, _ := client.BaseURL.Parse(server.URL + "/")
	client.BaseURL = url

	g := NewGitHubExtensionWithClient(client.Client())
	g.client = client

	var files []FileInfo
	err := g.walkRepo(context.Background(), client, "owner", "repo", "", nil, &files)
	assert.Error(t, err)
}
