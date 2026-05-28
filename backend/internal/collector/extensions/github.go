package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/google/go-github/v63/github"
	"github.com/odysseythink/hermind/backend/internal/collector/core"
)

// supportedExts lists file extensions that the collector can parse.
var supportedExts = []string{
	".txt", ".md", ".markdown", ".rst", ".adoc",
	".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rb", ".java", ".c", ".cpp", ".h", ".hpp",
	".cs", ".php", ".swift", ".kt", ".rs", ".scala", ".r", ".m", ".mm", ".sh", ".bash",
	".zsh", ".ps1", ".bat", ".cmd", ".sql", ".html", ".htm", ".css", ".scss", ".sass",
	".less", ".xml", ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf",
	".pdf", ".docx", ".doc", ".odt", ".epub", ".pptx", ".ppt", ".odp", ".xlsx", ".xls", ".ods",
	".csv", ".tsv",
	".png", ".jpg", ".jpeg", ".webp",
	".mp3", ".wav", ".mp4", ".mpeg", ".ogg", ".oga", ".opus", ".m4a", ".webm",
}

func isSupportedExt(ext string) bool {
	ext = strings.ToLower(ext)
	for _, e := range supportedExts {
		if e == ext {
			return true
		}
	}
	return false
}

// RepoRequest is the common payload for repository extensions.
type RepoRequest struct {
	RepoURL     string `json:"repoUrl"`
	Branch      string `json:"branch,omitempty"`
	AccessToken string `json:"accessToken,omitempty"`
}

// FileInfo describes a single file retrieved from a repository.
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// GitHubExtension implements the Extension interface for GitHub repositories.
type GitHubExtension struct {
	client *github.Client
}

// NewGitHubExtension creates a new GitHubExtension.
func NewGitHubExtension() *GitHubExtension {
	return &GitHubExtension{client: github.NewClient(nil)}
}

// NewGitHubExtensionWithClient creates a new GitHubExtension with a custom HTTP client.
func NewGitHubExtensionWithClient(httpClient *http.Client) *GitHubExtension {
	return &GitHubExtension{client: github.NewClient(httpClient)}
}

// Name returns the extension name.
func (g *GitHubExtension) Name() string { return "github" }

// Handle routes GitHub extension requests.
func (g *GitHubExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}

	switch endpoint {
	case "/ext/github-repo":
		return g.loadRepo(ctx, body)
	case "/ext/github-repo/branches":
		return g.getBranches(ctx, body)
	default:
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
}

func (g *GitHubExtension) loadRepo(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req RepoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	owner, repo, err := parseGitHubURL(req.RepoURL)
	if err != nil {
		return nil, err
	}

	client := g.client
	if req.AccessToken != "" {
		tokenClient := github.NewClient(nil)
		tokenClient = tokenClient.WithAuthToken(req.AccessToken)
		client = tokenClient
	}

	opts := &github.RepositoryContentGetOptions{}
	if req.Branch != "" {
		opts.Ref = req.Branch
	}

	var files []FileInfo
	if err := g.walkRepo(ctx, client, owner, repo, "", opts, &files); err != nil {
		return nil, err
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"files": files},
	}, nil
}

func (g *GitHubExtension) walkRepo(ctx context.Context, client *github.Client, owner, repo, dir string, opts *github.RepositoryContentGetOptions, files *[]FileInfo) error {
	_, contents, _, err := client.Repositories.GetContents(ctx, owner, repo, dir, opts)
	if err != nil {
		return fmt.Errorf("get contents %s: %w", dir, err)
	}

	for _, c := range contents {
		if c.GetType() == "dir" {
			if err := g.walkRepo(ctx, client, owner, repo, c.GetPath(), opts, files); err != nil {
				return err
			}
			continue
		}
		if !isSupportedExt(path.Ext(c.GetName())) {
			continue
		}
		content, err := g.downloadContent(ctx, client, owner, repo, c.GetPath(), opts)
		if err != nil {
			continue // skip files we can't download
		}
		*files = append(*files, FileInfo{
			Name:    c.GetName(),
			Path:    c.GetPath(),
			Content: content,
		})
	}
	return nil
}

func (g *GitHubExtension) downloadContent(ctx context.Context, client *github.Client, owner, repo, filePath string, opts *github.RepositoryContentGetOptions) (string, error) {
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, filePath, opts)
	if err != nil {
		return "", err
	}
	if fileContent.GetEncoding() == "base64" {
		return fileContent.GetContent()
	}
	// Fallback: fetch raw download URL
	if fileContent.GetDownloadURL() != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileContent.GetDownloadURL(), nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Client().Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", fmt.Errorf("no content available for %s", filePath)
}

func (g *GitHubExtension) getBranches(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req RepoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	owner, repo, err := parseGitHubURL(req.RepoURL)
	if err != nil {
		return nil, err
	}

	client := g.client
	if req.AccessToken != "" {
		tokenClient := github.NewClient(nil)
		tokenClient = tokenClient.WithAuthToken(req.AccessToken)
		client = tokenClient
	}

	branches, _, err := client.Repositories.ListBranches(ctx, owner, repo, nil)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	var names []string
	for _, b := range branches {
		names = append(names, b.GetName())
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"branches": names},
	}, nil
}

func parseGitHubURL(repoURL string) (owner, repo string, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid GitHub repository URL")
	}
	return parts[0], parts[1], nil
}
