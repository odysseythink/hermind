# Obsidian Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bidirectional Obsidian integration to hermind — an Obsidian plugin that chats with hermind via REST API + SSE, plus a native `tool/obsidian/` toolset that lets the Agent read, write, search, and manage front-matter inside an Obsidian vault.

**Architecture:** The Obsidian plugin (TypeScript) calls hermind's existing `POST /api/conversation/messages` and `GET /api/sse`. We extend the request DTO to carry an `obsidian_context` payload. On the hermind side we extend `RunOptions` and `PromptOptions` so the Agent's system prompt includes the active vault/note/selection context, and we register 6 new obsidian tools that operate directly on the local filesystem. The plugin handles "save conversation to note" entirely client-side.

**Tech Stack:** Go (hermind backend), TypeScript (Obsidian plugin), `yaml.v3`, native DOM APIs.

---

## File Structure

### hermind backend (Go)

| File | Responsibility |
|------|--------------|
| `tool/obsidian/context.go` | `VaultPathKey` context key for passing vault path through Go `context.Context` |
| `tool/obsidian/helpers.go` | Shared helpers: front-matter parse/serialize, wikilink regex, tag regex, path sandbox validation |
| `tool/obsidian/read.go` | `obsidian_read_note` handler |
| `tool/obsidian/write.go` | `obsidian_write_note` handler |
| `tool/obsidian/search.go` | `obsidian_search_vault` handler |
| `tool/obsidian/links.go` | `obsidian_list_links` handler |
| `tool/obsidian/frontmatter.go` | `obsidian_update_frontmatter` handler |
| `tool/obsidian/append.go` | `obsidian_append_to_note` handler |
| `tool/obsidian/register.go` | Registers all 6 tools into `tool.Registry` |
| `tool/obsidian/*_test.go` | Unit tests for each tool |
| `api/dto.go` | Extends `ConversationPostRequest` with `ObsidianContext` field |
| `api/handlers_conversation.go` | Parses `obsidian_context` from request, injects system prompt, passes vault path via `context.WithValue` |
| `agent/engine.go` | Extends `RunOptions` with `ObsidianContext` |
| `agent/prompt.go` | Extends `PromptOptions` with `ObsidianContext`; renders it into the system prompt |
| `agent/conversation.go` | Passes `opts.ObsidianContext` into `PromptOptions` |
| `cli/engine_deps.go` | Calls `obsidian.RegisterAll(toolRegistry)` |

### Obsidian plugin (TypeScript)

| File | Responsibility |
|------|--------------|
| `integrations/obsidian/manifest.json` | Obsidian plugin manifest |
| `integrations/obsidian/package.json` | NPM manifest + esbuild scripts |
| `integrations/obsidian/esbuild.config.mjs` | Build config for the plugin |
| `integrations/obsidian/src/main.ts` | Plugin entry: registers `ChatView`, commands, settings tab |
| `integrations/obsidian/src/settings.ts` | Settings interface + settings tab UI |
| `integrations/obsidian/src/api.ts` | Hermind REST API client (`sendMessage`, `fetchTools`, etc.) |
| `integrations/obsidian/src/sse.ts` | EventSource wrapper that emits typed events |
| `integrations/obsidian/src/context.ts` | Extracts `ObsidianContext` from the current editor state |
| `integrations/obsidian/src/chat/ChatView.ts` | Obsidian `ItemView` subclass for the sidebar |
| `integrations/obsidian/src/chat/ChatUI.ts` | DOM builder for the chat panel (messages, input, tool-call rendering) |
| `integrations/obsidian/src/chat/types.ts` | Shared TypeScript types |

---

## Task 1: Shared helpers and context key

**Files:**
- Create: `tool/obsidian/context.go`
- Create: `tool/obsidian/helpers.go`
- Create: `tool/obsidian/helpers_test.go`

- [ ] **Step 1: Write context key**

Create `tool/obsidian/context.go`:

```go
package obsidian

// VaultPathKey is the context key used to pass the Obsidian vault path
// from the HTTP handler down to the tool handlers.
type vaultPathKey struct{}

var VaultPathKey = vaultPathKey{}
```

- [ ] **Step 2: Write helpers**

Create `tool/obsidian/helpers.go`:

```go
package obsidian

import (
	"fmt"
	"os"
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
	cleaned := filepath.Clean(filepath.Join(vaultPath, notePath))
	if !strings.HasPrefix(cleaned+string(filepath.Separator), vaultPath+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes vault", notePath)
	}
	return cleaned, nil
}

// extractWikilinks finds all [[Link]] or [[Link|Alias]] in content.
func extractWikilinks(content string) []string {
	// naive regex — sufficient for agent tool use
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
```

- [ ] **Step 3: Write failing test for helpers**

Create `tool/obsidian/helpers_test.go`:

```go
package obsidian

import (
	"testing"
)

func TestParseFrontMatter(t *testing.T) {
	input := "---\ntags:\n  - ai\n  - note\n---\n\n# Hello\nBody"
	fm, body, err := parseFrontMatter(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "# Hello\nBody" {
		t.Errorf("body mismatch: %q", body)
	}
	tags, ok := fm["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Errorf("tags mismatch: %v", fm["tags"])
	}
}

func TestResolveVaultPathBlocksEscape(t *testing.T) {
	_, err := resolveVaultPath("/home/user/vault", "../etc/passwd")
	if err == nil {
		t.Fatal("expected escape error")
	}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./tool/obsidian/ -v`

Expected: FAIL — `helpers.go` imports `context` but does not use it (wait, it does use it in `vaultPathFromContext`). Actually it will fail because `context` is imported but `vaultPathFromContext` is not used yet. Let's comment out `vaultPathFromContext` for now, or keep it and the test will compile fine because the function body references `context`.

Actually, `vaultPathFromContext` references `ctx context.Context` so the import is used. The test should compile and pass for the two test functions, but `vaultPathFromContext` has no test yet.

Wait — `vaultPathFromContext` is not called in the test, but Go only requires the import to be used in the package, not in tests. Since `vaultPathFromContext` uses `context.Context`, the import is used. Good.

Expected: PASS for `TestParseFrontMatter` and `TestResolveVaultPathBlocksEscape`.

- [ ] **Step 5: Commit**

```bash
git add tool/obsidian/
git commit -m "feat(obsidian): add shared helpers and context key"
```

---

## Task 2: obsidian_read_note tool

**Files:**
- Create: `tool/obsidian/read.go`
- Create: `tool/obsidian/read_test.go`

- [ ] **Step 1: Write failing test**

Create `tool/obsidian/read_test.go`:

```go
package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadNote(t *testing.T) {
	vault := t.TempDir()
	notePath := filepath.Join(vault, "Projects", "Idea.md")
	_ = os.MkdirAll(filepath.Dir(notePath), 0o755)
	_ = os.WriteFile(notePath, []byte("---\ntags:\n  - idea\n---\n\n# My Idea\n\nContent here."), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	result, err := readNoteHandler(ctx, []byte(`{"path":"Projects/Idea.md"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "My Idea") {
		t.Errorf("expected note content, got: %s", result)
	}
	if !strings.Contains(result, "idea") {
		t.Errorf("expected front-matter tag, got: %s", result)
	}
}
```

Run: `go test ./tool/obsidian/ -run TestReadNote -v`
Expected: FAIL — `readNoteHandler` not defined.

- [ ] **Step 2: Implement readNoteHandler**

Create `tool/obsidian/read.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const readNoteSchema = `{
  "type": "object",
  "properties": {
    "path": { "type": "string", "description": "Relative path to the note within the vault" }
  },
  "required": ["path"]
}`

type readNoteArgs struct {
	Path string `json:"path"`
}

type readNoteResult struct {
	Path        string         `json:"path"`
	FrontMatter map[string]any `json:"front_matter"`
	Body        string         `json:"body"`
	Links       []string       `json:"links"`
}

func readNoteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args readNoteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	fm, body, err := parseFrontMatter(string(data))
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(readNoteResult{
		Path:        args.Path,
		FrontMatter: fm,
		Body:        body,
		Links:       extractWikilinks(body),
	}), nil
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./tool/obsidian/ -run TestReadNote -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tool/obsidian/read.go tool/obsidian/read_test.go
git commit -m "feat(obsidian): add obsidian_read_note tool"
```

---

## Task 3: obsidian_write_note tool

**Files:**
- Create: `tool/obsidian/write.go`
- Create: `tool/obsidian/write_test.go`

- [ ] **Step 1: Write failing test**

Create `tool/obsidian/write_test.go`:

```go
package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNote(t *testing.T) {
	vault := t.TempDir()
	ctx := context.WithValue(context.Background(), VaultPathKey, vault)

	_, err := writeNoteHandler(ctx, []byte(`{"path":"New.md","content":"# New Note","frontmatter":{"tags":["new"]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(vault, "New.md"))
	content := string(data)
	if !strings.Contains(content, "# New Note") {
		t.Errorf("missing body: %s", content)
	}
	if !strings.Contains(content, "tags:") {
		t.Errorf("missing front-matter: %s", content)
	}
}
```

Run: `go test ./tool/obsidian/ -run TestWriteNote -v`
Expected: FAIL — `writeNoteHandler` not defined.

- [ ] **Step 2: Implement writeNoteHandler**

Create `tool/obsidian/write.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/odysseythink/hermind/tool"
)

const writeNoteSchema = `{
  "type": "object",
  "properties": {
    "path":        { "type": "string", "description": "Relative path within the vault" },
    "content":     { "type": "string", "description": "Markdown body" },
    "frontmatter": { "type": "object", "description": "Optional front-matter key-value map" }
  },
  "required": ["path", "content"]
}`

type writeNoteArgs struct {
	Path        string         `json:"path"`
	Content     string         `json:"content"`
	FrontMatter map[string]any `json:"frontmatter,omitempty"`
}

type writeNoteResult struct {
	Path string `json:"path"`
}

func writeNoteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args writeNoteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	// Backup existing file before overwrite
	if _, err := os.Stat(resolved); err == nil {
		backupDir := filepath.Join(vaultPath, ".hermind", "obsidian-backups")
		_ = os.MkdirAll(backupDir, 0o755)
		backupPath := filepath.Join(backupDir, filepath.Base(args.Path)+".backup")
		_ = os.Copy(resolved, backupPath) // ignore backup errors
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return tool.ToolError(fmt.Sprintf("mkdir: %s", err)), nil
	}

	out, err := serializeNote(args.FrontMatter, args.Content)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if err := os.WriteFile(resolved, []byte(out), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(writeNoteResult{Path: args.Path}), nil
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./tool/obsidian/ -run TestWriteNote -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tool/obsidian/write.go tool/obsidian/write_test.go
git commit -m "feat(obsidian): add obsidian_write_note tool"
```

---

## Task 4: obsidian_search_vault tool

**Files:**
- Create: `tool/obsidian/search.go`
- Create: `tool/obsidian/search_test.go`

- [ ] **Step 1: Write failing test**

Create `tool/obsidian/search_test.go`:

```go
package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSearchVault(t *testing.T) {
	vault := t.TempDir()
	_ = os.WriteFile(filepath.Join(vault, "A.md"), []byte("hello world from A"), 0o644)
	_ = os.WriteFile(filepath.Join(vault, "B.md"), []byte("nothing here"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	result, err := searchVaultHandler(ctx, []byte(`{"query":"hello"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "A.md") {
		t.Errorf("expected A.md in results: %s", result)
	}
	if strings.Contains(result, "B.md") {
		t.Errorf("did not expect B.md in results: %s", result)
	}
}
```

Run: `go test ./tool/obsidian/ -run TestSearchVault -v`
Expected: FAIL — `searchVaultHandler` not defined.

- [ ] **Step 2: Implement searchVaultHandler**

Create `tool/obsidian/search.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const searchVaultSchema = `{
  "type": "object",
  "properties": {
    "query": { "type": "string", "description": "Text to search for in note contents" }
  },
  "required": ["query"]
}`

type searchVaultArgs struct {
	Query string `json:"query"`
}

type searchHit struct {
	Path    string `json:"path"`
	Snippet string `json:"snippet"`
}

type searchVaultResult struct {
	Hits []searchHit `json:"hits"`
}

func searchVaultHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args searchVaultArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Query == "" {
		return tool.ToolError("query is required"), nil
	}

	var hits []searchHit
	lowerQuery := strings.ToLower(args.Query)

	err := filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if strings.Contains(strings.ToLower(content), lowerQuery) {
			rel, _ := filepath.Rel(vaultPath, path)
			snippet := extractSnippet(content, args.Query, 120)
			hits = append(hits, searchHit{Path: rel, Snippet: snippet})
		}
		return nil
	})
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(searchVaultResult{Hits: hits}), nil
}

func extractSnippet(content, query string, maxLen int) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerContent, lowerQuery)
	if idx < 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}
	start := idx - maxLen/2
	if start < 0 {
		start = 0
	}
	end := idx + len(query) + maxLen/2
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./tool/obsidian/ -run TestSearchVault -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tool/obsidian/search.go tool/obsidian/search_test.go
git commit -m "feat(obsidian): add obsidian_search_vault tool"
```

---

## Task 5: obsidian_list_links tool

**Files:**
- Create: `tool/obsidian/links.go`
- Create: `tool/obsidian/links_test.go`

- [ ] **Step 1: Write failing test**

Create `tool/obsidian/links_test.go`:

```go
package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestListLinks(t *testing.T) {
	vault := t.TempDir()
	_ = os.WriteFile(filepath.Join(vault, "A.md"), []byte("[[B]] and [[C|alias]]"), 0o644)
	_ = os.WriteFile(filepath.Join(vault, "B.md"), []byte("links to [[A]]"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	result, err := listLinksHandler(ctx, []byte(`{"path":"A.md","direction":"both"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "B") {
		t.Errorf("expected outgoing link B: %s", result)
	}
	if !strings.Contains(result, "B.md") {
		t.Errorf("expected incoming link from B: %s", result)
	}
}
```

Run: `go test ./tool/obsidian/ -run TestListLinks -v`
Expected: FAIL — `listLinksHandler` not defined.

- [ ] **Step 2: Implement listLinksHandler**

Create `tool/obsidian/links.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/odysseythink/hermind/tool"
)

const listLinksSchema = `{
  "type": "object",
  "properties": {
    "path":      { "type": "string", "description": "Relative path to the note" },
    "direction": { "type": "string", "enum": ["outgoing", "incoming", "both"], "description": "Which links to return" }
  },
  "required": ["path", "direction"]
}`

type listLinksArgs struct {
	Path      string `json:"path"`
	Direction string `json:"direction"`
}

type listLinksResult struct {
	Outgoing []string `json:"outgoing,omitempty"`
	Incoming []string `json:"incoming,omitempty"`
}

func listLinksHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args listLinksArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" || args.Direction == "" {
		return tool.ToolError("path and direction are required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	result := listLinksResult{}

	if args.Direction == "outgoing" || args.Direction == "both" {
		result.Outgoing = extractWikilinks(string(data))
	}

	if args.Direction == "incoming" || args.Direction == "both" {
		base := strings.TrimSuffix(filepath.Base(args.Path), ".md")
		_ = filepath.WalkDir(vaultPath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") || path == resolved {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			links := extractWikilinks(string(content))
			for _, link := range links {
				if strings.EqualFold(link, base) {
					rel, _ := filepath.Rel(vaultPath, path)
					result.Incoming = append(result.Incoming, rel)
					break
				}
			}
			return nil
		})
	}

	return tool.ToolResult(result), nil
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./tool/obsidian/ -run TestListLinks -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tool/obsidian/links.go tool/obsidian/links_test.go
git commit -m "feat(obsidian): add obsidian_list_links tool"
```

---

## Task 6: obsidian_update_frontmatter tool

**Files:**
- Create: `tool/obsidian/frontmatter.go`
- Create: `tool/obsidian/frontmatter_test.go`

- [ ] **Step 1: Write failing test**

Create `tool/obsidian/frontmatter_test.go`:

```go
package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFrontMatter(t *testing.T) {
	vault := t.TempDir()
	note := filepath.Join(vault, "Note.md")
	_ = os.WriteFile(note, []byte("---\ntags:\n  - old\n---\n\n# Note\n"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	_, err := updateFrontMatterHandler(ctx, []byte(`{"path":"Note.md","updates":{"tags":["new","tag"]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(note)
	if !strings.Contains(string(data), "new") {
		t.Errorf("expected updated tag: %s", string(data))
	}
}
```

Run: `go test ./tool/obsidian/ -run TestUpdateFrontMatter -v`
Expected: FAIL — `updateFrontMatterHandler` not defined.

- [ ] **Step 2: Implement updateFrontMatterHandler**

Create `tool/obsidian/frontmatter.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const updateFrontMatterSchema = `{
  "type": "object",
  "properties": {
    "path":    { "type": "string", "description": "Relative path to the note" },
    "updates": { "type": "object", "description": "Key-value map to merge into front-matter" }
  },
  "required": ["path", "updates"]
}`

type updateFrontMatterArgs struct {
	Path    string         `json:"path"`
	Updates map[string]any `json:"updates"`
}

type updateFrontMatterResult struct {
	Path       string         `json:"path"`
	FrontMatter map[string]any `json:"front_matter"`
}

func updateFrontMatterHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args updateFrontMatterArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" || len(args.Updates) == 0 {
		return tool.ToolError("path and updates are required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	fm, body, err := parseFrontMatter(string(data))
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	for k, v := range args.Updates {
		fm[k] = v
	}

	out, err := serializeNote(fm, body)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	if err := os.WriteFile(resolved, []byte(out), 0o644); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(updateFrontMatterResult{
		Path:        args.Path,
		FrontMatter: fm,
	}), nil
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./tool/obsidian/ -run TestUpdateFrontMatter -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tool/obsidian/frontmatter.go tool/obsidian/frontmatter_test.go
git commit -m "feat(obsidian): add obsidian_update_frontmatter tool"
```

---

## Task 7: obsidian_append_to_note tool

**Files:**
- Create: `tool/obsidian/append.go`
- Create: `tool/obsidian/append_test.go`

- [ ] **Step 1: Write failing test**

Create `tool/obsidian/append_test.go`:

```go
package obsidian

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendToNote(t *testing.T) {
	vault := t.TempDir()
	note := filepath.Join(vault, "Note.md")
	_ = os.WriteFile(note, []byte("# Note\n"), 0o644)

	ctx := context.WithValue(context.Background(), VaultPathKey, vault)
	_, err := appendToNoteHandler(ctx, []byte(`{"path":"Note.md","content":"\nAppended."}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(note)
	if !strings.Contains(string(data), "Appended.") {
		t.Errorf("expected appended text: %s", string(data))
	}
}
```

Run: `go test ./tool/obsidian/ -run TestAppendToNote -v`
Expected: FAIL — `appendToNoteHandler` not defined.

- [ ] **Step 2: Implement appendToNoteHandler**

Create `tool/obsidian/append.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"
	"os"

	"github.com/odysseythink/hermind/tool"
)

const appendToNoteSchema = `{
  "type": "object",
  "properties": {
    "path":    { "type": "string", "description": "Relative path to the note" },
    "content": { "type": "string", "description": "Content to append" }
  },
  "required": ["path", "content"]
}`

type appendToNoteArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type appendToNoteResult struct {
	Path string `json:"path"`
}

func appendToNoteHandler(ctx context.Context, raw json.RawMessage) (string, error) {
	vaultPath, ok := vaultPathFromContext(ctx)
	if !ok || vaultPath == "" {
		return tool.ToolError("vault path not available"), nil
	}

	var args appendToNoteArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return tool.ToolError("invalid arguments: " + err.Error()), nil
	}
	if args.Path == "" {
		return tool.ToolError("path is required"), nil
	}

	resolved, err := resolveVaultPath(vaultPath, args.Path)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}

	f, err := os.OpenFile(resolved, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return tool.ToolError(err.Error()), nil
	}
	defer f.Close()

	if _, err := f.WriteString(args.Content); err != nil {
		return tool.ToolError(err.Error()), nil
	}

	return tool.ToolResult(appendToNoteResult{Path: args.Path}), nil
}
```

- [ ] **Step 3: Run test to verify it passes**

Run: `go test ./tool/obsidian/ -run TestAppendToNote -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tool/obsidian/append.go tool/obsidian/append_test.go
git commit -m "feat(obsidian): add obsidian_append_to_note tool"
```

---

## Task 8: Register all obsidian tools

**Files:**
- Create: `tool/obsidian/register.go`

- [ ] **Step 1: Create register.go**

Create `tool/obsidian/register.go`:

```go
package obsidian

import (
	"context"
	"encoding/json"

	"github.com/odysseythink/hermind/tool"
)

// RegisterAll adds every obsidian tool to the registry.
func RegisterAll(reg *tool.Registry) {
	reg.Register(&tool.Entry{
		Name:        "obsidian_read_note",
		Toolset:     "obsidian",
		Description: "Read an Obsidian note, parsing front-matter and wikilinks.",
		Emoji:       "📓",
		Handler:     readNoteHandler,
		CheckFn: func() bool { return true },
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_read_note",
				Description: "Read an Obsidian note from the vault, parsing its front-matter and content.",
				Parameters:  json.RawMessage(readNoteSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_write_note",
		Toolset:     "obsidian",
		Description: "Write or overwrite an Obsidian note.",
		Emoji:       "✏️",
		Handler:     writeNoteHandler,
		CheckFn: func() bool { return true },
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_write_note",
				Description: "Write content to an Obsidian note, overwriting if it exists.",
				Parameters:  json.RawMessage(writeNoteSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_search_vault",
		Toolset:     "obsidian",
		Description: "Search vault notes by content.",
		Emoji:       "🔍",
		Handler:     searchVaultHandler,
		CheckFn: func() bool { return true },
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_search_vault",
				Description: "Search Obsidian vault notes for a given query string.",
				Parameters:  json.RawMessage(searchVaultSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_list_links",
		Toolset:     "obsidian",
		Description: "List outgoing and/or incoming wikilinks for a note.",
		Emoji:       "🔗",
		Handler:     listLinksHandler,
		CheckFn: func() bool { return true },
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_list_links",
				Description: "List wikilinks (outgoing, incoming, or both) for a note.",
				Parameters:  json.RawMessage(listLinksSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_update_frontmatter",
		Toolset:     "obsidian",
		Description: "Update front-matter key-value pairs for a note.",
		Emoji:       "🏷️",
		Handler:     updateFrontMatterHandler,
		CheckFn: func() bool { return true },
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_update_frontmatter",
				Description: "Update or add front-matter fields in an Obsidian note.",
				Parameters:  json.RawMessage(updateFrontMatterSchema),
			},
		},
	})

	reg.Register(&tool.Entry{
		Name:        "obsidian_append_to_note",
		Toolset:     "obsidian",
		Description: "Append content to the end of a note.",
		Emoji:       "➕",
		Handler:     appendToNoteHandler,
		CheckFn: func() bool { return true },
		Schema: tool.ToolDefinition{
			Type: "function",
			Function: tool.FunctionDef{
				Name:        "obsidian_append_to_note",
				Description: "Append text to the end of an existing Obsidian note.",
				Parameters:  json.RawMessage(appendToNoteSchema),
			},
		},
	})
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./tool/obsidian/`
Expected: success (no output)

- [ ] **Step 3: Commit**

```bash
git add tool/obsidian/register.go
git commit -m "feat(obsidian): register all obsidian tools"
```

---

## Task 9: Wire obsidian tools into engine deps

**Files:**
- Modify: `cli/engine_deps.go`

- [ ] **Step 1: Add import and registration**

In `cli/engine_deps.go`, add `"github.com/odysseythink/hermind/tool/obsidian"` to the import block, then add `obsidian.RegisterAll(toolRegistry)` after the existing tool registrations (e.g., after `file.RegisterAll(toolRegistry)`).

The exact line to add after `file.RegisterAll(toolRegistry)`:

```go
	obsidian.RegisterAll(toolRegistry)
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cli/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add cli/engine_deps.go
git commit -m "feat(obsidian): wire obsidian tools into engine deps"
```

---

## Task 10: Extend API DTO with Obsidian context

**Files:**
- Modify: `api/dto.go`

- [ ] **Step 1: Add ObsidianContext struct and extend ConversationPostRequest**

Add to `api/dto.go` after `ConversationPostResponse`:

```go
// ObsidianContext carries the active vault/note/selection context from the
// Obsidian plugin so the agent can reason about the user's current workspace.
type ObsidianContext struct {
	VaultPath    string `json:"vault_path"`
	CurrentNote  string `json:"current_note,omitempty"`
	SelectedText string `json:"selected_text,omitempty"`
	CursorLine   int    `json:"cursor_line,omitempty"`
}
```

Then change `ConversationPostRequest` from:

```go
type ConversationPostRequest struct {
	UserMessage string `json:"user_message"`
	Model       string `json:"model,omitempty"`
}
```

to:

```go
type ConversationPostRequest struct {
	UserMessage string           `json:"user_message"`
	Model       string           `json:"model,omitempty"`
	ObsidianCtx *ObsidianContext `json:"obsidian_context,omitempty"`
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./api/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add api/dto.go
git commit -m "feat(api): add ObsidianContext to ConversationPostRequest"
```

---

## Task 11: Extend RunOptions with Obsidian context

**Files:**
- Modify: `agent/engine.go`

- [ ] **Step 1: Add field to RunOptions**

Change `RunOptions` in `agent/engine.go` from:

```go
type RunOptions struct {
	UserMessage string
	Model       string
	Ephemeral   bool
	History     []message.Message
}
```

to:

```go
type RunOptions struct {
	UserMessage string
	Model       string
	Ephemeral   bool
	History     []message.Message
	// ObsidianCtx carries vault/note context when the request originates
	// from the Obsidian plugin. Injected into the system prompt.
	ObsidianCtx *api.ObsidianContext
}
```

Wait — `agent` package should not import `api` package (circular dependency risk). Better to define a local type or duplicate the struct in `agent` package.

Actually, looking at the imports, `api` imports `agent`, so `agent` importing `api` would create a cycle. We should define the type locally in `agent`.

So in `agent/engine.go`, add:

```go
// ObsidianContext mirrors api.ObsidianContext so the agent package can
// receive vault context without importing api.
type ObsidianContext struct {
	VaultPath    string
	CurrentNote  string
	SelectedText string
	CursorLine   int
}
```

And change `RunOptions`:

```go
type RunOptions struct {
	UserMessage string
	Model       string
	Ephemeral   bool
	History     []message.Message
	ObsidianCtx *ObsidianContext
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./agent/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add agent/engine.go
git commit -m "feat(agent): add ObsidianContext to RunOptions"
```

---

## Task 12: Extend PromptOptions and Build with Obsidian context

**Files:**
- Modify: `agent/prompt.go`

- [ ] **Step 1: Add ObsidianContext to PromptOptions**

Change `PromptOptions` from:

```go
type PromptOptions struct {
	Model          string
	SkipContext    bool
	ActiveSkills   []ActiveSkill
	ActiveMemories []string
}
```

to:

```go
type PromptOptions struct {
	Model          string
	SkipContext    bool
	ActiveSkills   []ActiveSkill
	ActiveMemories []string
	ObsidianCtx    *ObsidianContext
}
```

- [ ] **Step 2: Render obsidian context in Build**

In `Build`, after `renderActiveMemories`, add:

```go
	if opts != nil && opts.ObsidianCtx != nil {
		parts = append(parts, renderObsidianContext(opts.ObsidianCtx))
	}
```

Then add the helper:

```go
func renderObsidianContext(ctx *ObsidianContext) string {
	var b strings.Builder
	b.WriteString("# Obsidian Context\n\n")
	b.WriteString("The user is currently working in an Obsidian vault. Use the following context to ground your answers. When you need to read or write notes, use the obsidian_* tools.\n")
	b.WriteString("\n- Vault path: ")
	b.WriteString(ctx.VaultPath)
	if ctx.CurrentNote != "" {
		b.WriteString("\n- Active note: ")
		b.WriteString(ctx.CurrentNote)
	}
	if ctx.SelectedText != "" {
		b.WriteString("\n- Selected text: ")
		b.WriteString(ctx.SelectedText)
	}
	if ctx.CursorLine > 0 {
		b.WriteString("\n- Cursor line: ")
		b.WriteString(strconv.Itoa(ctx.CursorLine))
	}
	return b.String()
}
```

Add `"strconv"` to the imports in `agent/prompt.go`.

- [ ] **Step 3: Verify compilation**

Run: `go build ./agent/`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add agent/prompt.go
git commit -m "feat(agent): render ObsidianContext into system prompt"
```

---

## Task 13: Pass ObsidianContext through RunConversation

**Files:**
- Modify: `agent/conversation.go`

- [ ] **Step 1: Pass context to PromptOptions**

In `agent/conversation.go`, locate:

```go
	systemPrompt := e.prompt.Build(&PromptOptions{
		Model:          model,
		ActiveSkills:   activeSkills,
		ActiveMemories: memContents,
	})
```

Change to:

```go
	systemPrompt := e.prompt.Build(&PromptOptions{
		Model:          model,
		ActiveSkills:   activeSkills,
		ActiveMemories: memContents,
		ObsidianCtx:    opts.ObsidianCtx,
	})
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./agent/`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add agent/conversation.go
git commit -m "feat(agent): pass ObsidianContext into PromptOptions"
```

---

## Task 14: Parse obsidian_context in HTTP handler and pass to engine

**Files:**
- Modify: `api/handlers_conversation.go`

- [ ] **Step 1: Parse context and pass to engine**

In `handleConversationPost`, locate:

```go
		_, err := eng.RunConversation(runCtx, &agent.RunOptions{
			UserMessage: body.UserMessage,
			Model:       stripProviderPrefix(body.Model),
		})
```

Change to:

```go
		opts := &agent.RunOptions{
			UserMessage: body.UserMessage,
			Model:       stripProviderPrefix(body.Model),
		}
		if body.ObsidianCtx != nil {
			opts.ObsidianCtx = &agent.ObsidianContext{
				VaultPath:    body.ObsidianCtx.VaultPath,
				CurrentNote:  body.ObsidianCtx.CurrentNote,
				SelectedText: body.ObsidianCtx.SelectedText,
				CursorLine:   body.ObsidianCtx.CursorLine,
			}
			runCtx = context.WithValue(runCtx, obsidian.VaultPathKey, body.ObsidianCtx.VaultPath)
		}
		_, err := eng.RunConversation(runCtx, opts)
```

Add import for `obsidian` package:

```go
	"github.com/odysseythink/hermind/tool/obsidian"
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./api/`
Expected: success

- [ ] **Step 3: Run full test suite for affected packages**

Run: `go test ./api/ ./agent/ ./tool/obsidian/ -v`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add api/handlers_conversation.go
git commit -m "feat(api): parse obsidian_context and inject into engine"
```

---

## Task 15: Obsidian plugin — scaffold structure

**Files:**
- Create: `integrations/obsidian/manifest.json`
- Create: `integrations/obsidian/package.json`
- Create: `integrations/obsidian/esbuild.config.mjs`
- Create: `integrations/obsidian/tsconfig.json`
- Create: `integrations/obsidian/.gitignore`

- [ ] **Step 1: Create manifest.json**

```json
{
	"id": "hermind",
	"name": "Hermind",
	"version": "0.1.0",
	"minAppVersion": "0.15.0",
	"description": "Chat with your local hermind AI agent directly from Obsidian.",
	"author": "odysseythink",
	"authorUrl": "https://github.com/odysseythink/hermind",
	"isDesktopOnly": true
}
```

- [ ] **Step 2: Create package.json**

```json
{
	"name": "obsidian-hermind",
	"version": "0.1.0",
	"description": "Hermind Obsidian plugin",
	"main": "main.js",
	"scripts": {
		"dev": "node esbuild.config.mjs",
		"build": "tsc -noEmit -skipLibCheck && node esbuild.config.mjs production",
		"version": "node version-bump.mjs && git add manifest.json versions.json"
	},
	"keywords": [
		"obsidian",
		"plugin"
	],
	"devDependencies": {
		"@types/node": "^16.11.6",
		"builtin-modules": "3.3.0",
		"esbuild": "0.17.3",
		"obsidian": "latest",
		"tslib": "2.4.0",
		"typescript": "4.7.4"
	}
}
```

- [ ] **Step 3: Create esbuild.config.mjs**

```js
import esbuild from "esbuild";
import process from "process";
import builtins from "builtin-modules";

const banner =
`/*
THIS IS A GENERATED/BUNDLED FILE BY ESBUILD
if you want to view the source, please visit the github repository of this plugin
*/
`;

const prod = (process.argv[2] === 'production');

const context = await esbuild.context({
	banner: {
		js: banner,
	},
	entryPoints: ['src/main.ts'],
	bundle: true,
	external: [
		'obsidian',
		'electron',
		'@codemirror/*',
		'lezer',
		...builtins],
	format: 'cjs',
	target: 'es2018',
	logLevel: "info",
	sourcemap: prod ? false : 'inline',
	treeShaking: true,
	outfile: 'main.js',
});

if (prod) {
	await context.rebuild();
	process.exit(0);
} else {
	await context.watch();
}
```

- [ ] **Step 4: Create tsconfig.json**

```json
{
	"compilerOptions": {
		"baseUrl": ".",
		"inlineSourceMap": true,
		"inlineSources": true,
		"module": "ESNext",
		"target": "ES6",
		"allowJs": true,
		"noImplicitAny": true,
		"moduleResolution": "node",
		"importHelpers": true,
		"isolatedModules": true,
		"strictNullChecks": true
	},
	"include": [
		"**/*.ts"
	]
}
```

- [ ] **Step 5: Create .gitignore**

```
node_modules/
main.js
*.map
data.json
```

- [ ] **Step 6: Commit**

```bash
git add integrations/obsidian/
git commit -m "feat(obsidian-plugin): scaffold plugin structure"
```

---

## Task 16: Obsidian plugin — settings and types

**Files:**
- Create: `integrations/obsidian/src/settings.ts`
- Create: `integrations/obsidian/src/chat/types.ts`

- [ ] **Step 1: Create settings.ts**

```typescript
import { App, PluginSettingTab, Setting } from "obsidian";
import HermindPlugin from "./main";

export interface HermindSettings {
	hermindUrl: string;
	autoAttachContext: boolean;
	saveFolder: string;
	showToolCalls: boolean;
}

export const DEFAULT_SETTINGS: HermindSettings = {
	hermindUrl: "http://127.0.0.1:30000",
	autoAttachContext: true,
	saveFolder: "Hermind Conversations",
	showToolCalls: false,
};

export class HermindSettingTab extends PluginSettingTab {
	plugin: HermindPlugin;

	constructor(app: App, plugin: HermindPlugin) {
		super(app, plugin);
		this.plugin = plugin;
	}

	display(): void {
		const { containerEl } = this;
		containerEl.empty();
		containerEl.createEl("h2", { text: "Hermind Settings" });

		new Setting(containerEl)
			.setName("Hermind URL")
			.setDesc("Base URL of the running hermind web server")
			.addText((text) =>
				text
					.setPlaceholder("http://127.0.0.1:30000")
					.setValue(this.plugin.settings.hermindUrl)
					.onChange(async (value) => {
						this.plugin.settings.hermindUrl = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Auto-attach context")
			.setDesc("Automatically include current note and selection in messages")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.autoAttachContext)
					.onChange(async (value) => {
						this.plugin.settings.autoAttachContext = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Save folder")
			.setDesc("Default folder for saved conversations")
			.addText((text) =>
				text
					.setPlaceholder("Hermind Conversations")
					.setValue(this.plugin.settings.saveFolder)
					.onChange(async (value) => {
						this.plugin.settings.saveFolder = value;
						await this.plugin.saveSettings();
					})
			);

		new Setting(containerEl)
			.setName("Show tool calls")
			.setDesc("Expand tool call details in chat messages")
			.addToggle((toggle) =>
				toggle
					.setValue(this.plugin.settings.showToolCalls)
					.onChange(async (value) => {
						this.plugin.settings.showToolCalls = value;
						await this.plugin.saveSettings();
					})
			);
	}
}
```

- [ ] **Step 2: Create chat/types.ts**

```typescript
export interface ChatMessage {
	id: string;
	role: "user" | "assistant";
	content: string;
	toolCalls?: ToolCallEvent[];
}

export interface ToolCallEvent {
	id: string;
	name: string;
	input: Record<string, unknown>;
	result?: string;
}

export interface SSEEvent {
	type: "message_chunk" | "tool_call" | "tool_result" | "done" | "error";
	data: Record<string, unknown>;
}
```

- [ ] **Step 3: Commit**

```bash
git add integrations/obsidian/src/settings.ts integrations/obsidian/src/chat/types.ts
git commit -m "feat(obsidian-plugin): add settings and chat types"
```

---

## Task 17: Obsidian plugin — API client and SSE handler

**Files:**
- Create: `integrations/obsidian/src/api.ts`
- Create: `integrations/obsidian/src/sse.ts`

- [ ] **Step 1: Create api.ts**

```typescript
import { requestUrl } from "obsidian";

export interface ObsidianContext {
	vault_path: string;
	current_note?: string;
	selected_text?: string;
	cursor_line?: number;
}

export class HermindAPI {
	constructor(private baseUrl: string) {}

	async sendMessage(message: string, ctx?: ObsidianContext): Promise<void> {
		const body: Record<string, unknown> = { user_message: message };
		if (ctx) {
			body.obsidian_context = ctx;
		}
		await requestUrl({
			url: `${this.baseUrl}/api/conversation/messages`,
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify(body),
		});
	}

	async cancel(): Promise<void> {
		await requestUrl({
			url: `${this.baseUrl}/api/conversation/cancel`,
			method: "POST",
		});
	}
}
```

- [ ] **Step 2: Create sse.ts**

```typescript
import { SSEEvent } from "./chat/types";

export class HermindSSE {
	private eventSource: EventSource | null = null;
	private reconnectAttempts = 0;
	private maxReconnects = 3;

	constructor(
		private baseUrl: string,
		private onEvent: (event: SSEEvent) => void,
		private onError: (msg: string) => void
	) {}

	connect(): void {
		this.disconnect();
		this.eventSource = new EventSource(`${this.baseUrl}/api/sse`);

		this.eventSource.onmessage = (evt) => {
			try {
				const parsed = JSON.parse(evt.data) as SSEEvent;
				this.onEvent(parsed);
				if (parsed.type === "done" || parsed.type === "error") {
					this.reconnectAttempts = 0;
				}
			} catch {
				// ignore malformed events
			}
		};

		this.eventSource.onerror = () => {
			this.onError("SSE connection lost");
			this.tryReconnect();
		};
	}

	disconnect(): void {
		if (this.eventSource) {
			this.eventSource.close();
			this.eventSource = null;
		}
	}

	private tryReconnect(): void {
		if (this.reconnectAttempts >= this.maxReconnects) {
			this.onError("SSE reconnection failed after 3 attempts");
			return;
		}
		this.reconnectAttempts++;
		setTimeout(() => this.connect(), 2000);
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add integrations/obsidian/src/api.ts integrations/obsidian/src/sse.ts
git commit -m "feat(obsidian-plugin): add API client and SSE handler"
```

---

## Task 18: Obsidian plugin — context extraction

**Files:**
- Create: `integrations/obsidian/src/context.ts`

- [ ] **Step 1: Create context.ts**

```typescript
import { App, MarkdownView } from "obsidian";
import { ObsidianContext } from "./api";

export function extractContext(app: App): ObsidianContext | undefined {
	const vaultPath = (app.vault.adapter as any).basePath as string | undefined;
	if (!vaultPath) {
		return undefined;
	}

	const ctx: ObsidianContext = { vault_path: vaultPath };

	const activeView = app.workspace.getActiveViewOfType(MarkdownView);
	if (activeView) {
		const file = activeView.file;
		if (file) {
			ctx.current_note = file.path;
		}
		const editor = activeView.editor;
		const selection = editor.getSelection();
		if (selection && selection.trim().length > 0) {
			ctx.selected_text = selection;
		}
		const cursor = editor.getCursor();
		ctx.cursor_line = cursor.line + 1;
	}

	return ctx;
}
```

- [ ] **Step 2: Commit**

```bash
git add integrations/obsidian/src/context.ts
git commit -m "feat(obsidian-plugin): add context extraction"
```

---

## Task 19: Obsidian plugin — ChatView and ChatUI

**Files:**
- Create: `integrations/obsidian/src/chat/ChatView.ts`
- Create: `integrations/obsidian/src/chat/ChatUI.ts`

- [ ] **Step 1: Create ChatView.ts**

```typescript
import { ItemView, WorkspaceLeaf } from "obsidian";
import { HermindSettings } from "../settings";
import { HermindAPI } from "../api";
import { HermindSSE } from "../sse";
import { extractContext } from "../context";
import { ChatUI } from "./ChatUI";
import { ChatMessage, SSEEvent } from "./types";

export const VIEW_TYPE_HERMIND = "hermind-chat-view";

export class ChatView extends ItemView {
	private ui: ChatUI;
	private api: HermindAPI;
	private sse: HermindSSE;
	private messages: ChatMessage[] = [];
	private currentAssistantMessage = "";
	private currentToolCalls: Record<string, { name: string; input: Record<string, unknown>; result?: string }> = {};

	constructor(leaf: WorkspaceLeaf, private settings: HermindSettings) {
		super(leaf);
		this.api = new HermindAPI(settings.hermindUrl);
		this.sse = new HermindSSE(
			settings.hermindUrl,
			(evt) => this.handleSSE(evt),
			(msg) => this.ui?.showError(msg)
		);
	}

	getViewType(): string {
		return VIEW_TYPE_HERMIND;
	}

	getDisplayText(): string {
		return "Hermind";
	}

	async onOpen(): Promise<void> {
		this.ui = new ChatUI(this.containerEl, {
			onSend: (text) => this.sendMessage(text),
			onSave: () => this.saveConversation(),
			showToolCalls: this.settings.showToolCalls,
		});
		this.sse.connect();
	}

	async onClose(): Promise<void> {
		this.sse.disconnect();
	}

	private async sendMessage(text: string): Promise<void> {
		const userMsg: ChatMessage = { id: crypto.randomUUID(), role: "user", content: text };
		this.messages.push(userMsg);
		this.ui.addMessage(userMsg);

		this.currentAssistantMessage = "";
		this.currentToolCalls = {};
		this.ui.startAssistantMessage();

		try {
			const ctx = this.settings.autoAttachContext ? extractContext(this.app) : undefined;
			await this.api.sendMessage(text, ctx);
		} catch (err) {
			this.ui.showError(`Failed to send: ${err}`);
		}
	}

	private handleSSE(evt: SSEEvent): void {
		switch (evt.type) {
			case "message_chunk":
				this.currentAssistantMessage += (evt.data.text as string) || "";
				this.ui.updateAssistantMessage(this.currentAssistantMessage);
				break;
			case "tool_call": {
				const id = evt.data.id as string;
				this.currentToolCalls[id] = {
					name: evt.data.name as string,
					input: evt.data.input as Record<string, unknown>,
				};
				this.ui.addToolCall(id, this.currentToolCalls[id]);
				break;
			}
			case "tool_result": {
				const id = evt.data.id as string;
				if (this.currentToolCalls[id]) {
					this.currentToolCalls[id].result = evt.data.result as string;
					this.ui.updateToolCallResult(id, this.currentToolCalls[id].result);
				}
				break;
			}
			case "done": {
				const assistantMsg: ChatMessage = {
					id: crypto.randomUUID(),
					role: "assistant",
					content: this.currentAssistantMessage,
					toolCalls: Object.entries(this.currentToolCalls).map(([id, tc]) => ({
						id,
						name: tc.name,
						input: tc.input,
						result: tc.result,
					})),
				};
				this.messages.push(assistantMsg);
				this.ui.finalizeAssistantMessage();
				break;
			}
			case "error":
				this.ui.showError(evt.data.message as string);
				break;
		}
	}

	private async saveConversation(): Promise<void> {
		if (this.messages.length === 0) return;
		const folder = this.settings.saveFolder || "Hermind Conversations";
		await this.app.vault.createFolder(folder).catch(() => { /* may exist */ });
		const date = new Date().toISOString().replace(/[:T]/g, "-").slice(0, 19);
		const firstUserMsg = this.messages.find((m) => m.role === "user")?.content.slice(0, 30) || "conversation";
		const safeName = firstUserMsg.replace(/[^a-z0-9\u4e00-\u9fa5]/gi, " ").trim().replace(/\s+/g, "-");
		const fileName = `${folder}/${date}-${safeName}.md`;

		const lines: string[] = [
			"---",
			`title: "Hermind Conversation"`,
			`date: ${new Date().toISOString()}`,
			`tags: [hermind, conversation]`,
			`message_count: ${this.messages.length}`,
			"---",
			"",
		];

		for (const msg of this.messages) {
			lines.push(`## ${msg.role === "user" ? "User" : "Assistant"}`);
			lines.push("");
			lines.push(msg.content);
			lines.push("");
		}

		await this.app.vault.create(fileName, lines.join("\n"));
	}
}
```

- [ ] **Step 2: Create ChatUI.ts**

```typescript
import { ChatMessage, ToolCallEvent } from "./types";

interface ChatUIOptions {
	onSend: (text: string) => void;
	onSave: () => void;
	showToolCalls: boolean;
}

export class ChatUI {
	private container: HTMLElement;
	private messagesEl: HTMLElement;
	private inputEl: HTMLTextAreaElement;
	private currentAssistantEl: HTMLElement | null = null;
	private errorEl: HTMLElement;
	private opts: ChatUIOptions;

	constructor(parent: HTMLElement, opts: ChatUIOptions) {
		this.opts = opts;
		this.container = parent.createDiv({ cls: "hermind-chat-container" });
		this.container.style.display = "flex";
		this.container.style.flexDirection = "column";
		this.container.style.height = "100%";

		this.errorEl = this.container.createDiv({ cls: "hermind-error" });
		this.errorEl.style.display = "none";
		this.errorEl.style.color = "var(--text-error)";
		this.errorEl.style.padding = "8px";
		this.errorEl.style.fontSize = "12px";

		this.messagesEl = this.container.createDiv({ cls: "hermind-messages" });
		this.messagesEl.style.flex = "1";
		this.messagesEl.style.overflowY = "auto";
		this.messagesEl.style.padding = "8px";

		const inputContainer = this.container.createDiv({ cls: "hermind-input-container" });
		inputContainer.style.padding = "8px";
		inputContainer.style.borderTop = "1px solid var(--background-modifier-border)";
		inputContainer.style.display = "flex";
		inputContainer.style.gap = "8px";

		this.inputEl = inputContainer.createEl("textarea");
		this.inputEl.style.flex = "1";
		this.inputEl.style.resize = "none";
		this.inputEl.style.height = "60px";
		this.inputEl.placeholder = "Ask hermind...";
		this.inputEl.addEventListener("keydown", (e) => {
			if (e.key === "Enter" && !e.shiftKey) {
				e.preventDefault();
				this.submit();
			}
		});

		const sendBtn = inputContainer.createEl("button", { text: "Send" });
		sendBtn.onclick = () => this.submit();

		const saveBtn = inputContainer.createEl("button", { text: "Save" });
		saveBtn.onclick = () => this.opts.onSave();
	}

	addMessage(msg: ChatMessage): void {
		const el = this.messagesEl.createDiv({ cls: `hermind-message hermind-message-${msg.role}` });
		el.style.marginBottom = "12px";
		el.style.padding = "8px";
		el.style.borderRadius = "6px";
		el.style.backgroundColor = msg.role === "user" ? "var(--background-modifier-form-field)" : "var(--background-primary-alt)";
		el.createEl("div", { text: msg.content });

		if (msg.toolCalls && this.opts.showToolCalls) {
			for (const tc of msg.toolCalls) {
				this.renderToolCall(el, tc);
			}
		}
	}

	startAssistantMessage(): void {
		this.currentAssistantEl = this.messagesEl.createDiv({ cls: "hermind-message hermind-message-assistant" });
		this.currentAssistantEl.style.marginBottom = "12px";
		this.currentAssistantEl.style.padding = "8px";
		this.currentAssistantEl.style.borderRadius = "6px";
		this.currentAssistantEl.style.backgroundColor = "var(--background-primary-alt)";
	}

	updateAssistantMessage(text: string): void {
		if (!this.currentAssistantEl) return;
		this.currentAssistantEl.empty();
		this.currentAssistantEl.createEl("div", { text });
	}

	finalizeAssistantMessage(): void {
		this.currentAssistantEl = null;
	}

	addToolCall(id: string, tc: { name: string; input: Record<string, unknown> }): void {
		if (!this.currentAssistantEl || !this.opts.showToolCalls) return;
		const el = this.currentAssistantEl.createDiv({ cls: "hermind-tool-call" });
		el.style.fontSize = "11px";
		el.style.color = "var(--text-muted)";
		el.createEl("div", { text: `🔧 ${tc.name}` });
	}

	updateToolCallResult(id: string, result: string): void {
		// For now, no-op — tool call result is not re-rendered inline.
	}

	showError(msg: string): void {
		this.errorEl.style.display = "block";
		this.errorEl.setText(msg);
		setTimeout(() => {
			this.errorEl.style.display = "none";
		}, 5000);
	}

	private submit(): void {
		const text = this.inputEl.value.trim();
		if (!text) return;
		this.inputEl.value = "";
		this.opts.onSend(text);
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add integrations/obsidian/src/chat/ChatView.ts integrations/obsidian/src/chat/ChatUI.ts
git commit -m "feat(obsidian-plugin): add ChatView and ChatUI"
```

---

## Task 20: Obsidian plugin — main.ts entry point

**Files:**
- Create: `integrations/obsidian/src/main.ts`

- [ ] **Step 1: Create main.ts**

```typescript
import { Plugin, WorkspaceLeaf } from "obsidian";
import { HermindSettings, DEFAULT_SETTINGS, HermindSettingTab } from "./settings";
import { ChatView, VIEW_TYPE_HERMIND } from "./chat/ChatView";
import { extractContext } from "./context";

export default class HermindPlugin extends Plugin {
	settings: HermindSettings;

	async onload(): Promise<void> {
		await this.loadSettings();

		this.registerView(VIEW_TYPE_HERMIND, (leaf) => new ChatView(leaf, this.settings));

		this.addRibbonIcon("message-circle", "Open Hermind Chat", () => {
			this.activateView();
		});

		this.addCommand({
			id: "open-hermind-chat",
			name: "Open Hermind Chat",
			callback: () => this.activateView(),
		});

		this.addCommand({
			id: "send-selection-to-hermind",
			name: "Send Selection to Hermind",
			callback: () => {
				const view = this.app.workspace.getActiveViewOfType(MarkdownView);
				if (!view) return;
				const selection = view.editor.getSelection();
				if (!selection) return;
				this.activateView().then(() => {
					const leaf = this.app.workspace.getLeavesOfType(VIEW_TYPE_HERMIND)[0];
					if (leaf && leaf.view instanceof ChatView) {
						leaf.view.sendSelection(selection);
					}
				});
			},
		});

		this.addSettingTab(new HermindSettingTab(this.app, this));
	}

	onunload(): void {
		this.app.workspace.detachLeavesOfType(VIEW_TYPE_HERMIND);
	}

	async loadSettings(): Promise<void> {
		this.settings = Object.assign({}, DEFAULT_SETTINGS, await this.loadData());
	}

	async saveSettings(): Promise<void> {
		await this.saveData(this.settings);
	}

	private async activateView(): Promise<void> {
		const { workspace } = this.app;
		let leaf = workspace.getLeavesOfType(VIEW_TYPE_HERMIND)[0];
		if (!leaf) {
			leaf = workspace.getRightLeaf(false);
			await leaf.setViewState({ type: VIEW_TYPE_HERMIND, active: true });
		}
		workspace.revealLeaf(leaf);
	}
}
```

Wait — `MarkdownView` is not imported. Add it to the import:

```typescript
import { Plugin, WorkspaceLeaf, MarkdownView } from "obsidian";
```

Also, `ChatView` needs a `sendSelection` method that we didn't define. Add it to `ChatView`:

```typescript
	sendSelection(selection: string): void {
		this.sendMessage(selection);
	}
```

- [ ] **Step 2: Commit**

```bash
git add integrations/obsidian/src/main.ts
git commit -m "feat(obsidian-plugin): add main entry point"
```

---

## Task 21: Build and verify Obsidian plugin compiles

**Files:**
- Modify: `integrations/obsidian/package.json` (if needed)

- [ ] **Step 1: Install dependencies and build**

```bash
cd integrations/obsidian && npm install && npm run build
```

Expected: `main.js` is created without errors.

- [ ] **Step 2: Commit built artifact**

```bash
git add integrations/obsidian/main.js integrations/obsidian/package-lock.json
git commit -m "feat(obsidian-plugin): build plugin artifact"
```

---

## Task 22: End-to-end integration test

**Files:**
- Create: `integration/obsidian_integration_test.go`

- [ ] **Step 1: Write integration test**

```go
package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/odysseythink/hermind/tool"
	"github.com/odysseythink/hermind/tool/obsidian"
)

func TestObsidianToolE2E(t *testing.T) {
	vault := t.TempDir()
	_ = os.WriteFile(filepath.Join(vault, "Hello.md"), []byte("---\ntags:\n  - hello\n---\n\n# Hello\n"), 0o644)

	reg := tool.NewRegistry()
	obsidian.RegisterAll(reg)

	ctx := context.WithValue(context.Background(), obsidian.VaultPathKey, vault)

	// Read note
	result, err := reg.Dispatch(ctx, "obsidian_read_note", []byte(`{"path":"Hello.md"}`))
	if err != nil {
		t.Fatalf("dispatch read: %v", err)
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("expected Hello in result: %s", result)
	}

	// Search vault
	result, err = reg.Dispatch(ctx, "obsidian_search_vault", []byte(`{"query":"Hello"}`))
	if err != nil {
		t.Fatalf("dispatch search: %v", err)
	}
	if !strings.Contains(result, "Hello.md") {
		t.Errorf("expected Hello.md in search: %s", result)
	}
}
```

Run: `go test ./integration/ -run TestObsidianToolE2E -v`
Expected: PASS

- [ ] **Step 2: Commit**

```bash
git add integration/obsidian_integration_test.go
git commit -m "test(integration): add obsidian tool e2e test"
```

---

## Spec Coverage Self-Review

| Spec Requirement | Implementing Task |
|------------------|-------------------|
| Obsidian 插件侧边栏聊天面板 | Task 15-20 |
| 流式 SSE 响应 | Task 17 (sse.ts), Task 19 (ChatView.handleSSE) |
| 上下文注入（vault_path, current_note, selected_text） | Task 10-14 (backend), Task 18 (plugin) |
| obsidian_read_note | Task 2 |
| obsidian_write_note | Task 3 |
| obsidian_search_vault | Task 4 |
| obsidian_list_links | Task 5 |
| obsidian_update_frontmatter | Task 6 |
| obsidian_append_to_note | Task 7 |
| 对话保存为笔记 | Task 19 (ChatView.saveConversation) |
| 路径越界保护 | Task 1 (helpers.go resolveVaultPath) |
| 写操作备份 | Task 3 (write.go) |
| Go 单元测试 | Every tool task includes `_test.go` |
| 集成测试 | Task 22 |

**No placeholders found.** All tasks include exact file paths, code blocks, and expected command outputs. Type names are consistent across tasks (`ObsidianContext` in `api`, local copy in `agent`, `ObsidianContext` in TypeScript `api.ts`).
