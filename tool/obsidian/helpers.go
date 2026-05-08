package obsidian

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// parseFrontMatter splits a markdown file into front-matter (YAML) and body.
// If no front-matter is present, returns empty map and the full content as body.
func parseFrontMatter(content string) (map[string]any, string, error) {
	const sep = "---\n"
	if !strings.HasPrefix(content, sep) {
		return map[string]any{}, content, nil
	}
	rest := content[len(sep):]
	idx := strings.Index(rest, sep)
	if idx < 0 {
		return map[string]any{}, content, nil
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(rest[:idx]), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid front-matter: %w", err)
	}
	body := strings.TrimPrefix(rest[idx+len(sep):], "\n")
	return fm, body, nil
}

// serializeNote assembles front-matter and body back into markdown.
func serializeNote(fm map[string]any, body string) (string, error) {
	if len(fm) == 0 {
		return body, nil
	}
	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}
	return "---\n" + string(data) + "---\n\n" + body, nil
}

// resolveVaultPath ensures path is inside vaultPath. Returns cleaned absolute path or error.
func resolveVaultPath(vaultPath, notePath string) (string, error) {
	vaultPath = filepath.Clean(vaultPath)
	cleaned := filepath.Clean(filepath.Join(vaultPath, notePath))
	rel, err := filepath.Rel(vaultPath, cleaned)
	if err != nil {
		return "", fmt.Errorf("path %q escapes vault", notePath)
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes vault", notePath)
	}
	if vaultPath == string(filepath.Separator) && notePath != "" && !filepath.IsLocal(notePath) {
		return "", fmt.Errorf("path %q escapes vault", notePath)
	}
	return cleaned, nil
}

// extractWikilinks finds all [[Link]] or [[Link|Alias]] in content.
func extractWikilinks(content string) []string {
	var out []string
	for {
		i := strings.Index(content, "[[")
		if i < 0 {
			break
		}
		j := strings.Index(content[i+2:], "]]")
		if j < 0 {
			break
		}
		link := content[i+2 : i+2+j]
		if pipe := strings.Index(link, "|"); pipe >= 0 {
			link = link[:pipe]
		}
		out = append(out, link)
		content = content[i+4+j:]
	}
	return out
}

// vaultPathFromContext reads the vault path injected by the HTTP handler.
func vaultPathFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(VaultPathKey).(string)
	return v, ok
}
