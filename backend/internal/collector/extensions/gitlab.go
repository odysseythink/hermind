package extensions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/odysseythink/hermind/backend/internal/collector/core"
	"github.com/xanzy/go-gitlab"
)

// GitLabExtension implements the Extension interface for GitLab repositories.
type GitLabExtension struct {
	client *gitlab.Client
}

// NewGitLabExtension creates a new GitLabExtension.
func NewGitLabExtension() *GitLabExtension {
	return &GitLabExtension{}
}

// NewGitLabExtensionWithClient creates a new GitLabExtension with a custom base URL and HTTP client.
func NewGitLabExtensionWithClient(baseURL string, httpClient *http.Client) *GitLabExtension {
	c, _ := gitlab.NewClient("", gitlab.WithBaseURL(baseURL), gitlab.WithHTTPClient(httpClient))
	return &GitLabExtension{client: c}
}

// Name returns the extension name.
func (g *GitLabExtension) Name() string { return "gitlab" }

// Handle routes GitLab extension requests.
func (g *GitLabExtension) Handle(ctx context.Context, endpoint string, method string, body []byte) (*core.ExtensionResponse, error) {
	if method != http.MethodPost {
		return nil, fmt.Errorf("method %s not supported", method)
	}

	switch endpoint {
	case "/ext/gitlab-repo":
		return g.loadRepo(ctx, body)
	case "/ext/gitlab-repo/branches":
		return g.getBranches(ctx, body)
	default:
		return nil, fmt.Errorf("unknown endpoint %s", endpoint)
	}
}

func (g *GitLabExtension) loadRepo(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req RepoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	projectPath, err := parseGitLabProjectPath(req.RepoURL)
	if err != nil {
		return nil, err
	}

	client := g.client
	if req.AccessToken != "" {
		c, _ := gitlab.NewClient(req.AccessToken)
		client = c
	}

	var files []FileInfo
	opts := &gitlab.ListTreeOptions{Recursive: gitlab.Bool(true)}
	if req.Branch != "" {
		opts.Ref = gitlab.String(req.Branch)
	}

	for {
		tree, resp, err := client.Repositories.ListTree(projectPath, opts)
		if err != nil {
			return nil, fmt.Errorf("list tree: %w", err)
		}
		for _, item := range tree {
			if item.Type != "blob" {
				continue
			}
			if !isSupportedExt(path.Ext(item.Name)) {
				continue
			}
			content, err := g.downloadFile(ctx, client, projectPath, item.Path, req.Branch)
			if err != nil {
				continue
			}
			files = append(files, FileInfo{
				Name:    item.Name,
				Path:    item.Path,
				Content: content,
			})
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"files": files},
	}, nil
}

func (g *GitLabExtension) downloadFile(ctx context.Context, client *gitlab.Client, projectPath, filePath, ref string) (string, error) {
	opts := &gitlab.GetFileOptions{}
	if ref != "" {
		opts.Ref = gitlab.String(ref)
	}
	file, _, err := client.RepositoryFiles.GetFile(projectPath, filePath, opts)
	if err != nil {
		return "", err
	}
	if file.Encoding == "base64" {
		return file.Content, nil
	}
	return file.Content, nil
}

func (g *GitLabExtension) getBranches(ctx context.Context, body []byte) (*core.ExtensionResponse, error) {
	var req RepoRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("invalid request body: %w", err)
	}

	projectPath, err := parseGitLabProjectPath(req.RepoURL)
	if err != nil {
		return nil, err
	}

	client := g.client
	if req.AccessToken != "" {
		c, _ := gitlab.NewClient(req.AccessToken)
		client = c
	}

	branches, _, err := client.Branches.ListBranches(projectPath, nil)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	var names []string
	for _, b := range branches {
		names = append(names, b.Name)
	}

	return &core.ExtensionResponse{
		Success: true,
		Data:    map[string]interface{}{"branches": names},
	}, nil
}

func parseGitLabProjectPath(repoURL string) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid GitLab repository URL")
	}
	p := strings.Trim(u.Path, "/")
	if !strings.HasSuffix(p, ".git") {
		return p, nil
	}
	return strings.TrimSuffix(p, ".git"), nil
}
