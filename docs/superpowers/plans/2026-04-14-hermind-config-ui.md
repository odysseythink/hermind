# Hermind Config UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a dual TUI + Web configuration editor for `hermind` that preserves YAML comments, drives both UIs from a shared schema, and launches automatically on first run.

**Architecture:** A new `config/editor` package owns the YAML AST and a declarative `Schema()`; two thin renderers (`cli/ui/config` with bubbletea, `cli/ui/webconfig` with net/http + embed.FS) turn fields into forms. The existing `cli/app.go` gets a first-run hook; a new `hermind config [--web]` subcommand replaces the current interactive setup path.

**Tech Stack:** Go 1.25, `gopkg.in/yaml.v3` (Node API, already a dep), `github.com/charmbracelet/bubbletea` + `bubbles` (already a dep), `net/http`, `embed`. Frontend is plain HTML/CSS/vanilla JS, no build step.

**Spec:** `docs/superpowers/specs/2026-04-14-hermind-config-ui-design.md`

**Repo layout after this plan:**
```
hermind/
  config/
    editor/              ← new
      doc.go             ← Doc struct, Load, Save
      dotpath.go         ← path resolution against yaml.Node
      set.go             ← Get/Set/Remove/SetBlock
      schema.go          ← Field struct + Schema() catalog
      editor_test.go
      schema_test.go
      testdata/
        commented.yaml
        minimal.yaml
  cli/
    app.go               ← modify: first-run hook
    config.go            ← new: `hermind config [--web]` subcommand
    root.go              ← modify: wire newConfigCmd
    ui/
      config/            ← new: TUI
        model.go
        update.go
        view.go
        editors.go
        model_test.go
      webconfig/         ← new: Web
        server.go
        handlers.go
        openbrowser.go
        server_test.go
        web/
          index.html
          app.css
          app.js
```

---

## Task 1: `config/editor` — Doc skeleton, Load, Save

**Files:**
- Create: `hermind/config/editor/doc.go`
- Create: `hermind/config/editor/editor_test.go`
- Create: `hermind/config/editor/testdata/commented.yaml`

- [ ] **Step 1: Write the failing test**

Create `hermind/config/editor/testdata/commented.yaml`:
```yaml
# top comment
model: anthropic/claude-opus-4-6  # inline
providers:
  anthropic:
    api_key: abc
```

Create `hermind/config/editor/editor_test.go`:
```go
package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSaveRoundTripPreservesComments(t *testing.T) {
	src, err := os.ReadFile("testdata/commented.yaml")
	if err != nil { t.Fatal(err) }

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, src, 0o644); err != nil { t.Fatal(err) }

	doc, err := Load(path)
	if err != nil { t.Fatalf("Load: %v", err) }
	if err := doc.Save(); err != nil { t.Fatalf("Save: %v", err) }

	out, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	got := string(out)
	for _, want := range []string{"# top comment", "# inline", "anthropic/claude-opus-4-6", "api_key: abc"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in saved output:\n%s", want, got)
		}
	}
}

func TestLoadMissingFileReturnsEmptyDoc(t *testing.T) {
	doc, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil { t.Fatalf("Load: %v", err) }
	if doc == nil { t.Fatal("Load returned nil doc") }
	if doc.Path() == "" { t.Error("Path() empty") }
}
```

- [ ] **Step 2: Run test to verify it fails**

```
cd hermind && go test ./config/editor/ -run TestLoadSave -v
```
Expected: compile failure — `Load`, `Save`, `Path` not defined.

- [ ] **Step 3: Implement `doc.go`**

Create `hermind/config/editor/doc.go`:
```go
// Package editor owns the YAML AST of hermind's config file. It exposes
// Get/Set/Remove operations that preserve comments, ordering, and blank
// lines, and a Schema() catalog that describes every editable field so
// both the TUI and Web config UIs can render forms from the same source.
package editor

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Doc is a mutable handle on a YAML config file. Zero value is not usable;
// obtain one via Load.
type Doc struct {
	root *yaml.Node
	path string
}

// Load parses the YAML file at path. If the file does not exist, returns
// a Doc with an empty mapping root so callers can populate it and Save.
func Load(path string) (*Doc, error) {
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	var root yaml.Node
	if len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &root); err != nil {
			return nil, err
		}
	} else {
		root = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		}}
	}
	return &Doc{root: &root, path: path}, nil
}

// Path returns the file path this Doc will Save to.
func (d *Doc) Path() string { return d.path }

// Save atomically writes the current AST back to disk.
func (d *Doc) Save() error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(d.root); err != nil { return err }
	if err := enc.Close(); err != nil { return err }

	dir := filepath.Dir(d.path)
	tmp, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil { return err }
	tmpName := tmp.Name()
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close(); os.Remove(tmpName); return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close(); os.Remove(tmpName); return err
	}
	if err := tmp.Close(); err != nil { os.Remove(tmpName); return err }
	return os.Rename(tmpName, d.path)
}
```

- [ ] **Step 4: Run test to verify it passes**

```
go test ./config/editor/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add config/editor/doc.go config/editor/editor_test.go config/editor/testdata/commented.yaml
git commit -m "feat(config/editor): Load/Save with comment-preserving YAML AST"
```

---

## Task 2: `config/editor` — dotPath resolution (Get)

**Files:**
- Create: `hermind/config/editor/dotpath.go`
- Modify: `hermind/config/editor/editor_test.go`

- [ ] **Step 1: Add failing test**

Append to `editor_test.go`:
```go
func TestGet(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	cases := []struct {
		path string; want string; ok bool
	}{
		{"model", "anthropic/claude-opus-4-6", true},
		{"providers.anthropic.api_key", "abc", true},
		{"missing", "", false},
		{"providers.anthropic.missing", "", false},
	}
	for _, tc := range cases {
		got, ok := doc.Get(tc.path)
		if ok != tc.ok || got != tc.want {
			t.Errorf("Get(%q) = (%q, %v); want (%q, %v)", tc.path, got, ok, tc.want, tc.ok)
		}
	}
}

func mustLoad(t *testing.T, src string) *Doc {
	t.Helper()
	data, err := os.ReadFile(src); if err != nil { t.Fatal(err) }
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, data, 0o644); err != nil { t.Fatal(err) }
	d, err := Load(p); if err != nil { t.Fatal(err) }
	return d
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./config/editor/ -run TestGet -v` → `doc.Get undefined`.

- [ ] **Step 3: Implement `dotpath.go`**

Create `hermind/config/editor/dotpath.go`:
```go
package editor

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// documentContent returns the first child of the DocumentNode wrapper,
// which is the actual mapping root. Callers should nil-check.
func (d *Doc) documentContent() *yaml.Node {
	if d.root == nil || d.root.Kind != yaml.DocumentNode || len(d.root.Content) == 0 {
		return nil
	}
	return d.root.Content[0]
}

// Get returns the scalar value at dotPath. The second return is false if
// the path does not resolve to a scalar.
func (d *Doc) Get(dotPath string) (string, bool) {
	n := d.lookupNode(dotPath)
	if n == nil || n.Kind != yaml.ScalarNode {
		return "", false
	}
	return n.Value, true
}

// lookupNode walks dotPath against the document mapping. Returns nil if
// any segment is missing or a non-map is traversed.
func (d *Doc) lookupNode(dotPath string) *yaml.Node {
	cur := d.documentContent()
	if cur == nil { return nil }
	for _, seg := range strings.Split(dotPath, ".") {
		if cur.Kind != yaml.MappingNode { return nil }
		found := false
		for i := 0; i < len(cur.Content); i += 2 {
			k, v := cur.Content[i], cur.Content[i+1]
			if k.Value == seg { cur = v; found = true; break }
		}
		if !found { return nil }
	}
	return cur
}
```

- [ ] **Step 4: Run tests**

`go test ./config/editor/ -v` → all PASS.

- [ ] **Step 5: Commit**

```
git add config/editor/dotpath.go config/editor/editor_test.go
git commit -m "feat(config/editor): dotPath resolution for Get"
```

---

## Task 3: `config/editor` — Set / Remove

**Files:**
- Create: `hermind/config/editor/set.go`
- Modify: `hermind/config/editor/editor_test.go`

- [ ] **Step 1: Add failing tests**

Append to `editor_test.go`:
```go
func TestSetExistingScalar(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	if err := doc.Set("providers.anthropic.api_key", "NEW"); err != nil { t.Fatal(err) }
	v, ok := doc.Get("providers.anthropic.api_key")
	if !ok || v != "NEW" { t.Fatalf("got (%q,%v)", v, ok) }
	if err := doc.Save(); err != nil { t.Fatal(err) }
	b, _ := os.ReadFile(doc.Path())
	if !strings.Contains(string(b), "# top comment") { t.Error("top comment lost") }
	if !strings.Contains(string(b), "api_key: NEW") { t.Errorf("new value missing:\n%s", b) }
}

func TestSetCreatesIntermediateMaps(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	if err := doc.Set("agent.compression.threshold", "0.6"); err != nil { t.Fatal(err) }
	v, ok := doc.Get("agent.compression.threshold")
	if !ok || v != "0.6" { t.Fatalf("got (%q,%v)", v, ok) }
}

func TestRemove(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	if err := doc.Remove("providers.anthropic.api_key"); err != nil { t.Fatal(err) }
	if _, ok := doc.Get("providers.anthropic.api_key"); ok { t.Error("still present") }
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./config/editor/ -run 'TestSet|TestRemove' -v` → undefined.

- [ ] **Step 3: Implement `set.go`**

Create `hermind/config/editor/set.go`:
```go
package editor

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Set assigns a scalar value at dotPath, creating intermediate mappings as
// needed. If the existing node at dotPath is a scalar, its Value and Tag
// are updated in place so line/column comments stick.
func (d *Doc) Set(dotPath string, value any) error {
	cur := d.documentContent()
	if cur == nil {
		d.root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		cur = d.root.Content[0]
	}
	segs := strings.Split(dotPath, ".")
	for i, seg := range segs {
		last := i == len(segs)-1
		if cur.Kind != yaml.MappingNode {
			return fmt.Errorf("editor: %s: segment %q traverses non-map", dotPath, seg)
		}
		idx := indexOfKey(cur, seg)
		if idx < 0 {
			// Append new key/value pair.
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			var v *yaml.Node
			if last {
				v = scalarFromAny(value)
			} else {
				v = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			}
			cur.Content = append(cur.Content, k, v)
			cur = v
			continue
		}
		v := cur.Content[idx+1]
		if last {
			if v.Kind == yaml.ScalarNode {
				sv := scalarFromAny(value)
				v.Tag = sv.Tag
				v.Value = sv.Value
				v.Style = sv.Style
			} else {
				cur.Content[idx+1] = scalarFromAny(value)
			}
			return nil
		}
		if v.Kind != yaml.MappingNode {
			return fmt.Errorf("editor: %s: segment %q exists but is not a map", dotPath, seg)
		}
		cur = v
	}
	return nil
}

// Remove deletes the key addressed by dotPath. Silently succeeds if the
// path does not exist.
func (d *Doc) Remove(dotPath string) error {
	segs := strings.Split(dotPath, ".")
	parent := d.documentContent()
	if parent == nil { return nil }
	for _, seg := range segs[:len(segs)-1] {
		idx := indexOfKey(parent, seg)
		if idx < 0 { return nil }
		parent = parent.Content[idx+1]
		if parent.Kind != yaml.MappingNode { return nil }
	}
	last := segs[len(segs)-1]
	idx := indexOfKey(parent, last)
	if idx < 0 { return nil }
	parent.Content = append(parent.Content[:idx], parent.Content[idx+2:]...)
	return nil
}

func indexOfKey(mapNode *yaml.Node, key string) int {
	for i := 0; i < len(mapNode.Content); i += 2 {
		if mapNode.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func scalarFromAny(v any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.ScalarNode}
	switch x := v.(type) {
	case string:
		n.Tag = "!!str"; n.Value = x
	case bool:
		n.Tag = "!!bool"
		if x { n.Value = "true" } else { n.Value = "false" }
	case int:
		n.Tag = "!!int"; n.Value = fmt.Sprintf("%d", x)
	case int64:
		n.Tag = "!!int"; n.Value = fmt.Sprintf("%d", x)
	case float64:
		n.Tag = "!!float"; n.Value = fmt.Sprintf("%v", x)
	default:
		n.Tag = "!!str"; n.Value = fmt.Sprintf("%v", x)
	}
	return n
}
```

- [ ] **Step 4: Run tests**

`go test ./config/editor/ -v` → PASS.

- [ ] **Step 5: Commit**

```
git add config/editor/set.go config/editor/editor_test.go
git commit -m "feat(config/editor): Set and Remove with intermediate-map creation"
```

---

## Task 4: `config/editor` — SetBlock (insert raw YAML fragment)

**Files:**
- Modify: `hermind/config/editor/set.go`
- Modify: `hermind/config/editor/editor_test.go`

- [ ] **Step 1: Add failing test**

Append to `editor_test.go`:
```go
func TestSetBlockAddsNewMapEntry(t *testing.T) {
	doc := mustLoad(t, "testdata/commented.yaml")
	frag := "provider: openai\napi_key: sk-xxx\nmodel: gpt-4o\n"
	if err := doc.SetBlock("providers.openai", frag); err != nil { t.Fatal(err) }
	if v, _ := doc.Get("providers.openai.model"); v != "gpt-4o" {
		t.Errorf("got %q", v)
	}
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./config/editor/ -run TestSetBlock -v` → `SetBlock undefined`.

- [ ] **Step 3: Implement `SetBlock`**

Append to `hermind/config/editor/set.go`:
```go
// SetBlock parses a YAML mapping fragment and attaches it as the value at
// dotPath, replacing anything already there.
func (d *Doc) SetBlock(dotPath, fragment string) error {
	var tmp yaml.Node
	if err := yaml.Unmarshal([]byte(fragment), &tmp); err != nil {
		return fmt.Errorf("editor: SetBlock %s: parse fragment: %w", dotPath, err)
	}
	if tmp.Kind != yaml.DocumentNode || len(tmp.Content) == 0 {
		return fmt.Errorf("editor: SetBlock %s: empty fragment", dotPath)
	}
	newNode := tmp.Content[0]

	cur := d.documentContent()
	if cur == nil {
		d.root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
		cur = d.root.Content[0]
	}
	segs := strings.Split(dotPath, ".")
	for i, seg := range segs {
		last := i == len(segs)-1
		if cur.Kind != yaml.MappingNode {
			return fmt.Errorf("editor: SetBlock %s: non-map in path", dotPath)
		}
		idx := indexOfKey(cur, seg)
		if last {
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			if idx < 0 {
				cur.Content = append(cur.Content, k, newNode)
			} else {
				cur.Content[idx+1] = newNode
			}
			return nil
		}
		if idx < 0 {
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: seg}
			v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			cur.Content = append(cur.Content, k, v)
			cur = v
		} else {
			cur = cur.Content[idx+1]
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

`go test ./config/editor/ -v` → all PASS.

- [ ] **Step 5: Commit**

```
git add config/editor/set.go config/editor/editor_test.go
git commit -m "feat(config/editor): SetBlock for inserting map fragments"
```

---

## Task 5: `config/editor` — Schema catalog

**Files:**
- Create: `hermind/config/editor/schema.go`
- Create: `hermind/config/editor/schema_test.go`

- [ ] **Step 1: Write failing test**

Create `hermind/config/editor/schema_test.go`:
```go
package editor

import (
	"strings"
	"testing"
)

func TestSchemaHasKeyFields(t *testing.T) {
	s := Schema()
	seen := map[string]Field{}
	for _, f := range s { seen[f.Path] = f }
	for _, p := range []string{
		"model",
		"terminal.backend",
		"storage.sqlite_path",
		"agent.max_turns",
		"agent.compression.threshold",
		"memory.provider",
		"browser.provider",
	} {
		if _, ok := seen[p]; !ok { t.Errorf("missing field %q", p) }
	}
}

func TestSchemaFieldsHaveLabelsAndSections(t *testing.T) {
	for _, f := range Schema() {
		if strings.TrimSpace(f.Label) == "" { t.Errorf("%s: empty Label", f.Path) }
		if strings.TrimSpace(f.Section) == "" { t.Errorf("%s: empty Section", f.Path) }
	}
}

func TestSchemaEnumValidateRejectsUnknown(t *testing.T) {
	for _, f := range Schema() {
		if f.Kind != KindEnum { continue }
		if f.Validate == nil { t.Errorf("%s: enum without Validate", f.Path); continue }
		if err := f.Validate("___not_a_value___"); err == nil {
			t.Errorf("%s: Validate accepted bogus value", f.Path)
		}
	}
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./config/editor/ -run TestSchema -v` → `Schema`, `Field`, `Kind*` undefined.

- [ ] **Step 3: Implement `schema.go`**

Create `hermind/config/editor/schema.go`:
```go
package editor

import "fmt"

// Kind enumerates the UI renderer used for a Field.
type Kind int

const (
	KindString Kind = iota
	KindInt
	KindFloat
	KindBool
	KindEnum
	KindSecret
	KindList
)

// Field describes a single editable setting. Both the TUI and Web UI
// render forms from Schema().
type Field struct {
	Path     string
	Label    string
	Help     string
	Kind     Kind
	Enum     []string
	Section  string
	Validate func(any) error
}

func enumValidator(allowed []string) func(any) error {
	return func(v any) error {
		s, ok := v.(string)
		if !ok { return fmt.Errorf("expected string, got %T", v) }
		for _, a := range allowed { if a == s { return nil } }
		return fmt.Errorf("value %q not in %v", s, allowed)
	}
}

// Schema returns the static field catalog. Order determines display order.
func Schema() []Field {
	return []Field{
		// --- Model ---
		{Path: "model", Label: "Active model", Section: "Model", Kind: KindString,
			Help: "provider/model-name, e.g. anthropic/claude-opus-4-6"},

		// --- Agent ---
		{Path: "agent.max_turns", Label: "Max turns", Section: "Agent", Kind: KindInt,
			Help: "Maximum tool-use iterations per prompt."},
		{Path: "agent.gateway_timeout", Label: "Gateway timeout (s)", Section: "Agent", Kind: KindInt},
		{Path: "agent.compression.enabled", Label: "Compression enabled", Section: "Agent", Kind: KindBool},
		{Path: "agent.compression.threshold", Label: "Compression threshold", Section: "Agent", Kind: KindFloat,
			Help: "Fraction of context length at which compression triggers (0.0–1.0)."},
		{Path: "agent.compression.target_ratio", Label: "Compression target ratio", Section: "Agent", Kind: KindFloat},
		{Path: "agent.compression.protect_last", Label: "Protect last N messages", Section: "Agent", Kind: KindInt},
		{Path: "agent.compression.max_passes", Label: "Max compression passes", Section: "Agent", Kind: KindInt},

		// --- Terminal ---
		{Path: "terminal.backend", Label: "Terminal backend", Section: "Terminal",
			Kind: KindEnum, Enum: []string{"local", "modal", "singularity"},
			Validate: enumValidator([]string{"local", "modal", "singularity"})},

		// --- Storage ---
		{Path: "storage.driver", Label: "Storage driver", Section: "Storage",
			Kind: KindEnum, Enum: []string{"sqlite"},
			Validate: enumValidator([]string{"sqlite"})},
		{Path: "storage.sqlite_path", Label: "SQLite path", Section: "Storage", Kind: KindString},

		// --- Memory ---
		{Path: "memory.provider", Label: "Memory provider", Section: "Memory",
			Kind: KindEnum, Enum: []string{"", "honcho", "mem0", "supermemory", "hindsight", "retaindb", "openviking", "byterover", "holographic"},
			Validate: enumValidator([]string{"", "honcho", "mem0", "supermemory", "hindsight", "retaindb", "openviking", "byterover", "holographic"})},
		{Path: "memory.honcho.api_key", Label: "Honcho API key", Section: "Memory", Kind: KindSecret},
		{Path: "memory.mem0.api_key", Label: "Mem0 API key", Section: "Memory", Kind: KindSecret},
		{Path: "memory.supermemory.api_key", Label: "Supermemory API key", Section: "Memory", Kind: KindSecret},

		// --- Browser ---
		{Path: "browser.provider", Label: "Browser provider", Section: "Browser",
			Kind: KindEnum, Enum: []string{"", "browserbase", "camofox"},
			Validate: enumValidator([]string{"", "browserbase", "camofox"})},
		{Path: "browser.browserbase.api_key", Label: "Browserbase API key", Section: "Browser", Kind: KindSecret},
		{Path: "browser.browserbase.project_id", Label: "Browserbase project ID", Section: "Browser", Kind: KindString},
		{Path: "browser.camofox.base_url", Label: "Camofox base URL", Section: "Browser", Kind: KindString},

		// --- Providers (list) ---
		{Path: "providers", Label: "Providers", Section: "Providers", Kind: KindList,
			Help: "Add, remove, or edit LLM provider credentials."},

		// --- MCP (list) ---
		{Path: "mcp.servers", Label: "MCP servers", Section: "MCP", Kind: KindList},
	}
}

// Sections returns distinct Section names in first-seen order.
func Sections() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range Schema() {
		if seen[f.Section] { continue }
		seen[f.Section] = true
		out = append(out, f.Section)
	}
	return out
}
```

- [ ] **Step 4: Run tests**

`go test ./config/editor/ -v` → all PASS.

- [ ] **Step 5: Commit**

```
git add config/editor/schema.go config/editor/schema_test.go
git commit -m "feat(config/editor): Schema catalog driving both UIs"
```

---

## Task 6: TUI skeleton — Model, keybindings, section nav

**Files:**
- Create: `hermind/cli/ui/config/model.go`
- Create: `hermind/cli/ui/config/update.go`
- Create: `hermind/cli/ui/config/view.go`
- Create: `hermind/cli/ui/config/model_test.go`

- [ ] **Step 1: Write failing test**

Create `hermind/cli/ui/config/model_test.go`:
```go
package configui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelLoadsDoc(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("model: anthropic/claude\n"), 0o644); err != nil { t.Fatal(err) }
	m, err := NewModel(p); if err != nil { t.Fatal(err) }
	if m.CurrentSection() == "" { t.Error("no section selected") }
}

func TestTabAdvancesSection(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte(""), 0o644)
	m, _ := NewModel(p)
	first := m.CurrentSection()
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m2.(Model).CurrentSection() == first {
		t.Error("tab did not advance section")
	}
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./cli/ui/config/ -v` → `NewModel`, `CurrentSection` undefined.

- [ ] **Step 3: Implement model/update/view**

Create `hermind/cli/ui/config/model.go`:
```go
// Package configui is the bubbletea TUI for editing ~/.hermind/config.yaml.
package configui

import (
	"github.com/odysseythink/hermind/config/editor"

	tea "github.com/charmbracelet/bubbletea"
)

// Model is the bubbletea Model for the config screen.
type Model struct {
	doc          *editor.Doc
	sections     []string
	sectionIdx   int
	fieldIdx     int
	editing      bool
	dirty        bool
	status       string
}

// NewModel loads path and returns an initial Model.
func NewModel(path string) (Model, error) {
	doc, err := editor.Load(path)
	if err != nil { return Model{}, err }
	return Model{doc: doc, sections: editor.Sections()}, nil
}

// CurrentSection returns the name of the currently selected section.
func (m Model) CurrentSection() string {
	if len(m.sections) == 0 { return "" }
	return m.sections[m.sectionIdx]
}

// fieldsInCurrentSection returns the Field list for the selected section.
func (m Model) fieldsInCurrentSection() []editor.Field {
	var out []editor.Field
	for _, f := range editor.Schema() {
		if f.Section == m.CurrentSection() { out = append(out, f) }
	}
	return out
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Run launches the TUI with the given config path.
func Run(path string) error {
	m, err := NewModel(path); if err != nil { return err }
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// RunFirstRun is like Run but seeds a minimal Doc if path is absent.
func RunFirstRun(path string) error {
	return Run(path) // Load already tolerates missing file.
}
```

Create `hermind/cli/ui/config/update.go`:
```go
package configui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok { return m, nil }

	if m.editing {
		return m.updateEditing(key)
	}

	switch key.Type {
	case tea.KeyTab:
		m.sectionIdx = (m.sectionIdx + 1) % len(m.sections)
		m.fieldIdx = 0
	case tea.KeyShiftTab:
		m.sectionIdx = (m.sectionIdx - 1 + len(m.sections)) % len(m.sections)
		m.fieldIdx = 0
	case tea.KeyUp:
		if m.fieldIdx > 0 { m.fieldIdx-- }
	case tea.KeyDown:
		if m.fieldIdx < len(m.fieldsInCurrentSection())-1 { m.fieldIdx++ }
	case tea.KeyEnter:
		m.editing = true
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "q":
			return m, tea.Quit
		case "s":
			if err := m.doc.Save(); err != nil {
				m.status = "save failed: " + err.Error()
			} else {
				m.dirty = false
				m.status = "saved. restart hermind to apply."
			}
		}
	}
	return m, nil
}

// updateEditing is filled in by Task 7 (field editors). For now, Esc cancels.
func (m Model) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Type == tea.KeyEsc { m.editing = false }
	return m, nil
}
```

Create `hermind/cli/ui/config/view.go`:
```go
package configui

import (
	"fmt"
	"strings"

	"github.com/odysseythink/hermind/config/editor"
)

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString("hermind config — [tab] section  [↑↓] field  [enter] edit  [s] save  [q] quit\n\n")

	// section column
	b.WriteString("Sections:\n")
	for i, s := range m.sections {
		marker := "  "
		if i == m.sectionIdx { marker = "> " }
		fmt.Fprintf(&b, "%s%s\n", marker, s)
	}
	b.WriteString("\nFields:\n")

	for i, f := range m.fieldsInCurrentSection() {
		marker := "  "
		if i == m.fieldIdx { marker = "> " }
		val, _ := m.doc.Get(f.Path)
		if f.Kind == editor.KindSecret && val != "" { val = "••••" }
		fmt.Fprintf(&b, "%s%-28s %s\n", marker, f.Label+":", val)
	}

	if help := m.currentFieldHelp(); help != "" {
		b.WriteString("\n")
		b.WriteString(help)
	}
	if m.status != "" {
		b.WriteString("\n")
		b.WriteString(m.status)
	}
	return b.String()
}

func (m Model) currentFieldHelp() string {
	fields := m.fieldsInCurrentSection()
	if m.fieldIdx >= len(fields) { return "" }
	return fields[m.fieldIdx].Help
}
```

- [ ] **Step 4: Run tests**

```
go test ./cli/ui/config/ -v
go build ./...
```
Both PASS.

- [ ] **Step 5: Commit**

```
git add cli/ui/config/
git commit -m "feat(cli/ui/config): TUI skeleton with section/field navigation"
```

---

## Task 7: TUI field editors (string/int/bool/enum/secret)

**Files:**
- Create: `hermind/cli/ui/config/editors.go`
- Modify: `hermind/cli/ui/config/model.go` (add editor state)
- Modify: `hermind/cli/ui/config/update.go` (wire editing)
- Modify: `hermind/cli/ui/config/model_test.go`

- [ ] **Step 1: Add failing test**

Append to `model_test.go`:
```go
func TestEditStringFieldWritesDoc(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("model: old\n"), 0o644)
	m, _ := NewModel(p)
	// navigate to "model" field (first field of "Model" section, already default)
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})         // enter edit
	for _, r := range "new-model" {
		m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m2, _ = m2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit
	got, _ := m2.(Model).doc.Get("model")
	if got != "new-model" { t.Errorf("got %q, want %q", got, "new-model") }
}
```

- [ ] **Step 2: Run to verify fail**

Expected: fails because editing just toggles a bool today and does not mutate Doc.

- [ ] **Step 3: Implement field editors**

Create `hermind/cli/ui/config/editors.go`:
```go
package configui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/odysseythink/hermind/config/editor"
)

// fieldEditor holds per-field editor state while Model.editing==true.
type fieldEditor struct {
	field editor.Field
	input textinput.Model
	enumIdx int
}

func newFieldEditor(f editor.Field, current string) fieldEditor {
	ti := textinput.New()
	ti.SetValue(current)
	ti.Focus()
	if f.Kind == editor.KindSecret { ti.EchoMode = textinput.EchoPassword }
	fe := fieldEditor{field: f, input: ti}
	if f.Kind == editor.KindEnum {
		for i, v := range f.Enum { if v == current { fe.enumIdx = i } }
	}
	return fe
}

// commit converts the editor state back into a value and writes it to doc.
// Returns user-visible error string or empty on success.
func (fe fieldEditor) commit(doc *editor.Doc) string {
	switch fe.field.Kind {
	case editor.KindBool:
		// toggled directly via updateEditing; still persist
		return writeField(doc, fe.field, strings.TrimSpace(fe.input.Value()))
	case editor.KindInt:
		s := strings.TrimSpace(fe.input.Value())
		if _, err := strconv.Atoi(s); err != nil { return "not an integer: " + err.Error() }
		return writeField(doc, fe.field, s)
	case editor.KindFloat:
		s := strings.TrimSpace(fe.input.Value())
		if _, err := strconv.ParseFloat(s, 64); err != nil { return "not a number: " + err.Error() }
		return writeField(doc, fe.field, s)
	case editor.KindEnum:
		return writeField(doc, fe.field, fe.field.Enum[fe.enumIdx])
	default: // String / Secret
		return writeField(doc, fe.field, fe.input.Value())
	}
}

func writeField(doc *editor.Doc, f editor.Field, v string) string {
	if f.Validate != nil {
		if err := f.Validate(v); err != nil { return err.Error() }
	}
	if err := doc.Set(f.Path, v); err != nil { return err.Error() }
	return ""
}
```

Append to `model.go`:
```go
// editor state — populated while Model.editing is true.
```

Replace the Model struct with:
```go
type Model struct {
	doc          *editor.Doc
	sections     []string
	sectionIdx   int
	fieldIdx     int
	editing      bool
	ed           *fieldEditor
	dirty        bool
	status       string
}
```

Replace `updateEditing` in `update.go`:
```go
func (m Model) updateEditing(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Type == tea.KeyEsc { m.editing = false; m.ed = nil; return m, nil }
	if m.ed == nil { return m, nil }

	switch m.ed.field.Kind {
	case editor.KindEnum:
		switch key.Type {
		case tea.KeyLeft, tea.KeyUp:
			if m.ed.enumIdx > 0 { m.ed.enumIdx-- }
		case tea.KeyRight, tea.KeyDown:
			if m.ed.enumIdx < len(m.ed.field.Enum)-1 { m.ed.enumIdx++ }
		case tea.KeyEnter:
			if errMsg := m.ed.commit(m.doc); errMsg != "" {
				m.status = errMsg
			} else {
				m.dirty = true; m.editing = false; m.ed = nil
			}
		}
	case editor.KindBool:
		if key.Type == tea.KeyEnter || key.Type == tea.KeySpace {
			cur := m.ed.input.Value()
			next := "true"
			if cur == "true" { next = "false" }
			m.ed.input.SetValue(next)
			if errMsg := m.ed.commit(m.doc); errMsg != "" {
				m.status = errMsg
			} else {
				m.dirty = true; m.editing = false; m.ed = nil
			}
		}
	default:
		if key.Type == tea.KeyEnter {
			if errMsg := m.ed.commit(m.doc); errMsg != "" {
				m.status = errMsg
			} else {
				m.dirty = true; m.editing = false; m.ed = nil
			}
			return m, nil
		}
		var cmd tea.Cmd
		m.ed.input, cmd = m.ed.input.Update(key)
		return m, cmd
	}
	return m, nil
}
```

Update the KeyEnter branch in the non-editing section of `update.go`:
```go
	case tea.KeyEnter:
		fields := m.fieldsInCurrentSection()
		if m.fieldIdx >= len(fields) { return m, nil }
		f := fields[m.fieldIdx]
		if f.Kind == editor.KindList { return m, nil } // list editing is Task 8
		cur, _ := m.doc.Get(f.Path)
		fe := newFieldEditor(f, cur)
		m.ed = &fe
		m.editing = true
```

Add to top of `update.go`:
```go
import "github.com/odysseythink/hermind/config/editor"
```

- [ ] **Step 4: Run tests**

```
go test ./cli/ui/config/ -v
```
PASS.

- [ ] **Step 5: Commit**

```
git add cli/ui/config/
git commit -m "feat(cli/ui/config): string/int/float/bool/enum/secret editors"
```

---

## Task 8: TUI list editor (providers)

**Files:**
- Modify: `hermind/cli/ui/config/editors.go`
- Modify: `hermind/cli/ui/config/update.go`
- Modify: `hermind/cli/ui/config/view.go`
- Modify: `hermind/cli/ui/config/model_test.go`

- [ ] **Step 1: Add failing test**

Append to `model_test.go`:
```go
func TestAddProvider(t *testing.T) {
	dir := t.TempDir(); p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("providers:\n  anthropic:\n    api_key: k\n"), 0o644)
	m, _ := NewModel(p)
	// navigate to Providers section
	for m.CurrentSection() != "Providers" {
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = m2.(Model)
	}
	// 'a' adds a blank provider named "new"
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = m2.(Model)
	// name it via editing prompt
	for _, r := range "openai" { m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}); m = m2.(Model) }
	m2, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}); m = m2.(Model)
	if _, ok := m.doc.Get("providers.openai.provider"); !ok {
		t.Error("new provider not created")
	}
}
```

- [ ] **Step 2: Run to verify fail**

`'a'` currently does nothing.

- [ ] **Step 3: Implement list handling**

Extend `fieldEditor` and add a list state mode in `editors.go`. Add to `editors.go`:
```go
// newProviderBlock returns the YAML fragment for a blank provider entry.
func newProviderBlock(name string) string {
	return "provider: " + name + "\napi_key: \"\"\nmodel: \"\"\n"
}
```

In `update.go`, non-editing branch, add a rune handler:
```go
		case "a":
			// Add new item for list section.
			fields := m.fieldsInCurrentSection()
			if m.fieldIdx >= len(fields) || fields[m.fieldIdx].Kind != editor.KindList {
				return m, nil
			}
			// Stash an "add-list-item" editor with a text prompt for the key.
			f := fields[m.fieldIdx]
			ti := textinput.New(); ti.Placeholder = "name (e.g. openai)"; ti.Focus()
			m.ed = &fieldEditor{field: editor.Field{Path: f.Path, Kind: editor.KindString, Label: "new " + f.Label}, input: ti}
			m.editing = true
			m.status = "enter new item key, enter to confirm"
```

Extend `commit` in `editors.go` so that when `field.Label` starts with `"new "` and the enclosing Path is a list, it writes via `SetBlock`:
```go
// Add to top of commit():
	if strings.HasPrefix(fe.field.Label, "new ") {
		name := strings.TrimSpace(fe.input.Value())
		if name == "" { return "name required" }
		if fe.field.Path == "providers" {
			return writeBlock(doc, fe.field.Path+"."+name, newProviderBlock(name))
		}
		if fe.field.Path == "mcp.servers" {
			return writeBlock(doc, fe.field.Path+"."+name, "command: \"\"\nargs: []\n")
		}
	}

// Helper appended to editors.go:
func writeBlock(doc *editor.Doc, path, frag string) string {
	if err := doc.SetBlock(path, frag); err != nil { return err.Error() }
	return ""
}
```

Add `import "github.com/charmbracelet/bubbles/textinput"` to `update.go`.

Also update `view.go` to render list items: in the fields loop, if `f.Kind == editor.KindList`, expand:
```go
		if f.Kind == editor.KindList {
			fmt.Fprintf(&b, "%s%-28s  [press 'a' to add, 'd' to delete]\n", marker, f.Label+":")
			n := m.doc // avoid shadowing
			for _, item := range m.listItems(f.Path) {
				fmt.Fprintf(&b, "    - %s\n", item)
			}
			_ = n
			continue
		}
```

Helper in `view.go`:
```go
func (m Model) listItems(path string) []string {
	n := m.doc
	_ = n
	// cheap: probe known keys by path-prefix via editor
	// Providers and mcp.servers are mappings, so we re-parse via GetKeys helper below.
	return m.doc.MapKeys(path)
}
```

Add to `editor/set.go`:
```go
// MapKeys returns the keys of the mapping at dotPath, or nil if the path
// is missing or not a mapping.
func (d *Doc) MapKeys(dotPath string) []string {
	n := d.lookupNode(dotPath)
	if n == nil || n.Kind != yaml.MappingNode { return nil }
	out := make([]string, 0, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 { out = append(out, n.Content[i].Value) }
	return out
}
```

- [ ] **Step 4: Run tests**

```
go test ./config/editor/ ./cli/ui/config/ -v
go build ./...
```
PASS.

- [ ] **Step 5: Commit**

```
git add config/editor/set.go cli/ui/config/
git commit -m "feat(cli/ui/config): list editor for providers and MCP servers"
```

---

## Task 9: `hermind config` subcommand wiring

**Files:**
- Create: `hermind/cli/config.go`
- Modify: `hermind/cli/root.go`

- [ ] **Step 1: Write config.go**

```go
package cli

import (
	"os"
	"path/filepath"

	"github.com/odysseythink/hermind/config"
	configui "github.com/odysseythink/hermind/cli/ui/config"
	"github.com/spf13/cobra"
)

func newConfigCmd(app *App) *cobra.Command {
	var web bool
	var port int
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Open the configuration editor (TUI by default, --web for browser)",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := defaultConfigPath()
			if err != nil { return err }
			if web {
				return serveWebConfig(path, port) // filled in Task 11
			}
			return configui.Run(path)
		},
	}
	cmd.Flags().BoolVar(&web, "web", false, "open the browser editor instead of the TUI")
	cmd.Flags().IntVar(&port, "port", 7777, "port for the --web editor")
	return cmd
}

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir(); if err != nil { return "", err }
	return filepath.Join(home, ".hermind", "config.yaml"), nil
}

// serveWebConfig is a stub until Task 10/11 replaces it.
func serveWebConfig(path string, port int) error {
	_ = config.DefaultConfigDir
	return cobra.CheckErr(os.ErrInvalid), nil // placeholder — never used before Task 11
}
```

(The placeholder in `serveWebConfig` is removed in Task 11; do not ship this as-is — Task 11 is a prerequisite before enabling the `--web` flag in any release tag.)

- [ ] **Step 2: Wire into `root.go`**

In `hermind/cli/root.go`, add `newConfigCmd(app)` to the `root.AddCommand(...)` list alongside `newSetupCmd(app)`.

- [ ] **Step 3: Build and smoke test**

```
go build -o bin/hermind ./cmd/hermind
./bin/hermind config --help
```
Expected: help text shows `--web` and `--port` flags.

- [ ] **Step 4: Manual smoke**

```
./bin/hermind config
```
Expected: TUI opens, shows sections from Schema(). `q` quits.

- [ ] **Step 5: Commit**

```
git add cli/config.go cli/root.go
git commit -m "feat(cli): add hermind config subcommand (TUI)"
```

---

## Task 10: First-run hook in `cli/app.go`

**Files:**
- Modify: `hermind/cli/app.go`

- [ ] **Step 1: Locate the config load**

Find where `App.NewApp()` calls `config.Load` (or equivalent). If it silently returns an empty config on missing file today, change it to detect the absence:

```go
func NewApp() (*App, error) {
	path, err := defaultConfigPath(); if err != nil { return nil, err }
	if _, statErr := os.Stat(path); errors.Is(statErr, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, "no config found — launching first-run setup...")
		if err := configui.RunFirstRun(path); err != nil {
			return nil, fmt.Errorf("first-run setup: %w", err)
		}
	}
	cfg, err := config.Load(path)
	if err != nil { return nil, err }
	// ...existing App construction...
}
```

(If `NewApp` already takes a path argument, thread `path` through instead of re-deriving it.)

- [ ] **Step 2: Build**

```
go build ./...
```
PASS.

- [ ] **Step 3: Manual verification**

```
mv ~/.hermind/config.yaml ~/.hermind/config.yaml.bak
./bin/hermind
```
Expected: "no config found" line, TUI opens. After `s`-save and `q`-quit, REPL boots. Restore:
```
mv ~/.hermind/config.yaml.bak ~/.hermind/config.yaml
```

- [ ] **Step 4: Commit**

```
git add cli/app.go
git commit -m "feat(cli): auto-launch first-run config TUI when ~/.hermind/config.yaml is absent"
```

---

## Task 11: Web backend — server + handlers

**Files:**
- Create: `hermind/cli/ui/webconfig/server.go`
- Create: `hermind/cli/ui/webconfig/handlers.go`
- Create: `hermind/cli/ui/webconfig/server_test.go`

- [ ] **Step 1: Write failing test**

Create `server_test.go`:
```go
package webconfig

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	os.WriteFile(p, []byte("model: old\n"), 0o644)
	s, err := New(p); if err != nil { t.Fatal(err) }
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, p
}

func TestGetConfig(t *testing.T) {
	ts, _ := newServer(t)
	resp, err := http.Get(ts.URL + "/api/config"); if err != nil { t.Fatal(err) }
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["model"] != "old" { t.Errorf("got %v", body["model"]) }
}

func TestPostConfigAndSave(t *testing.T) {
	ts, p := newServer(t)
	payload, _ := json.Marshal(map[string]any{"path": "model", "value": "new"})
	resp, err := http.Post(ts.URL+"/api/config", "application/json", bytes.NewReader(payload))
	if err != nil || resp.StatusCode != 200 { t.Fatalf("post: %v %v", err, resp.Status) }
	http.Post(ts.URL+"/api/save", "application/json", nil)
	raw, _ := os.ReadFile(p)
	if !bytes.Contains(raw, []byte("model: new")) { t.Errorf("file not updated:\n%s", raw) }
}

func TestGetConfigMasksSecrets(t *testing.T) {
	ts, p := newServer(t)
	os.WriteFile(p, []byte("providers:\n  anthropic:\n    api_key: supersecret\n"), 0o644)
	// reload
	s, _ := New(p)
	ts2 := httptest.NewServer(s.Handler()); defer ts2.Close()
	resp, _ := http.Get(ts2.URL + "/api/config")
	body, _ := json.Marshal(map[string]any{}); _ = body
	buf := new(bytes.Buffer); buf.ReadFrom(resp.Body)
	if bytes.Contains(buf.Bytes(), []byte("supersecret")) {
		t.Error("secret leaked")
	}
}
```

- [ ] **Step 2: Run to verify fail**

`go test ./cli/ui/webconfig/ -v` → `New`, `Handler` undefined.

- [ ] **Step 3: Implement server.go + handlers.go**

Create `server.go`:
```go
// Package webconfig serves a browser-based editor for ~/.hermind/config.yaml.
// It binds loopback-only and assumes a single-user machine: no auth.
package webconfig

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/odysseythink/hermind/config/editor"
)

//go:embed web/*
var webFS embed.FS

// Server wires editor.Doc to HTTP handlers + embedded static assets.
type Server struct { doc *editor.Doc }

// New loads path and prepares a Server.
func New(path string) (*Server, error) {
	doc, err := editor.Load(path); if err != nil { return nil, err }
	return &Server{doc: doc}, nil
}

// Handler returns the http.Handler for mounting.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(static)))
	mux.HandleFunc("/api/schema", s.handleSchema)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/save",   s.handleSave)
	mux.HandleFunc("/api/reveal", s.handleReveal)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)
	return mux
}

// Serve binds addr and serves until shutdown is requested.
func Serve(path, addr string) error {
	s, err := New(path); if err != nil { return err }
	return http.ListenAndServe(addr, s.Handler())
}
```

Create `handlers.go`:
```go
package webconfig

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/odysseythink/hermind/config/editor"
)

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, editor.Schema())
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		out := map[string]any{}
		for _, f := range editor.Schema() {
			if f.Kind == editor.KindList { continue }
			v, _ := s.doc.Get(f.Path)
			if f.Kind == editor.KindSecret && v != "" { v = "••••" }
			// store nested by path → last segment for easy JS consumption
			out[f.Path] = v
		}
		writeJSON(w, out)
	case http.MethodPost:
		var body struct { Path string `json:"path"`; Value any `json:"value"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), 400); return
		}
		for _, f := range editor.Schema() {
			if f.Path != body.Path { continue }
			if f.Validate != nil {
				if err := f.Validate(body.Value); err != nil {
					http.Error(w, err.Error(), 400); return
				}
			}
		}
		if err := s.doc.Set(body.Path, body.Value); err != nil {
			http.Error(w, err.Error(), 400); return
		}
		w.WriteHeader(204)
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	if err := s.doc.Save(); err != nil { http.Error(w, err.Error(), 500); return }
	w.WriteHeader(204)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	var body struct{ Path string `json:"path"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), 400); return
	}
	// Only allow reveal for fields declared as secret in the schema.
	for _, f := range editor.Schema() {
		if f.Path == body.Path && f.Kind == editor.KindSecret {
			v, _ := s.doc.Get(body.Path)
			writeJSON(w, map[string]string{"value": v})
			return
		}
	}
	http.Error(w, "not a secret field", 400)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	// Close the program by exiting the process; the caller must have
	// wrapped Serve in a goroutine and waited on a signal. The Web UI
	// calls this only when the user clicks "Save & Exit".
	w.WriteHeader(204)
	go func() { os.Exit(0) }()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

var _ = strings.HasPrefix // keep import even when unused in future refactors
```

- [ ] **Step 4: Run tests**

```
go test ./cli/ui/webconfig/ -v
```
PASS (the embed will find `web/` empty; add a placeholder `web/index.html` with a single line `<html><body>stub</body></html>` to satisfy the `//go:embed web/*` directive for now — Task 12 replaces it).

- [ ] **Step 5: Commit**

```
git add cli/ui/webconfig/
git commit -m "feat(cli/ui/webconfig): HTTP server with schema/config/save/reveal"
```

---

## Task 12: Web frontend — static HTML/CSS/JS

**Files:**
- Create: `hermind/cli/ui/webconfig/web/index.html`
- Create: `hermind/cli/ui/webconfig/web/app.css`
- Create: `hermind/cli/ui/webconfig/web/app.js`

- [ ] **Step 1: Create `index.html`**

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>hermind config</title>
  <link rel="stylesheet" href="/app.css">
</head>
<body>
  <aside id="sections"></aside>
  <main id="form"></main>
  <footer>
    <button id="save">Save</button>
    <button id="save-exit">Save &amp; Exit</button>
    <span id="status"></span>
  </footer>
  <script src="/app.js"></script>
</body>
</html>
```

- [ ] **Step 2: Create `app.css`**

```css
body { font-family: system-ui, sans-serif; display: grid; grid-template: 1fr 3rem / 14rem 1fr; height: 100vh; margin: 0; }
aside { background: #222; color: #eee; padding: 1rem; overflow-y: auto; }
aside div { cursor: pointer; padding: .25rem 0; }
aside div.active { font-weight: bold; }
main { padding: 1rem 2rem; overflow-y: auto; }
footer { grid-column: 1 / 3; background: #eee; padding: .5rem 1rem; display: flex; gap: .5rem; align-items: center; }
#status { color: #333; margin-left: auto; }
label { display: block; margin: .75rem 0; }
label span.lbl { display: inline-block; width: 18rem; font-weight: 500; }
label span.help { display: block; margin-left: 18rem; font-size: .8em; color: #666; }
input[type=text], input[type=number], input[type=password], select { min-width: 20rem; padding: .25rem; }
```

- [ ] **Step 3: Create `app.js`**

```js
let schema = [];
let values = {};
let currentSection = null;

async function boot() {
  schema = await (await fetch('/api/schema')).json();
  values = await (await fetch('/api/config')).json();
  const sections = [...new Set(schema.map(f => f.Section))];
  currentSection = sections[0];
  const nav = document.getElementById('sections');
  sections.forEach(name => {
    const d = document.createElement('div');
    d.textContent = name;
    d.onclick = () => { currentSection = name; renderForm(); renderNav(sections); };
    nav.appendChild(d);
  });
  renderNav(sections); renderForm();
  document.getElementById('save').onclick = () => save(false);
  document.getElementById('save-exit').onclick = () => save(true);
}

function renderNav(sections) {
  const nav = document.getElementById('sections');
  [...nav.children].forEach((d, i) => {
    d.classList.toggle('active', sections[i] === currentSection);
  });
}

function renderForm() {
  const main = document.getElementById('form');
  main.innerHTML = '';
  schema.filter(f => f.Section === currentSection).forEach(f => {
    const wrap = document.createElement('label');
    const lbl = document.createElement('span'); lbl.className = 'lbl'; lbl.textContent = f.Label;
    wrap.appendChild(lbl);
    wrap.appendChild(renderField(f));
    if (f.Help) { const h = document.createElement('span'); h.className = 'help'; h.textContent = f.Help; wrap.appendChild(h); }
    main.appendChild(wrap);
  });
}

function renderField(f) {
  const cur = values[f.Path] ?? '';
  if (f.Kind === 4 /* Enum */) {
    const sel = document.createElement('select');
    (f.Enum || []).forEach(v => { const o = document.createElement('option'); o.value = v; o.textContent = v || '(none)'; sel.appendChild(o); });
    sel.value = cur;
    sel.onchange = () => persist(f.Path, sel.value);
    return sel;
  }
  if (f.Kind === 3 /* Bool */) {
    const cb = document.createElement('input'); cb.type = 'checkbox';
    cb.checked = cur === 'true' || cur === true;
    cb.onchange = () => persist(f.Path, cb.checked ? 'true' : 'false');
    return cb;
  }
  if (f.Kind === 5 /* Secret */) {
    const box = document.createElement('span');
    const inp = document.createElement('input'); inp.type = 'password'; inp.value = cur;
    const btn = document.createElement('button'); btn.textContent = '👁'; btn.type = 'button';
    btn.onclick = async () => {
      if (inp.type === 'password') {
        const r = await fetch('/api/reveal', {method:'POST', body: JSON.stringify({path: f.Path})});
        if (r.ok) { const b = await r.json(); inp.value = b.value; inp.type = 'text'; }
      } else { inp.type = 'password'; }
    };
    inp.onchange = () => persist(f.Path, inp.value);
    box.appendChild(inp); box.appendChild(btn);
    return box;
  }
  const inp = document.createElement('input');
  inp.type = (f.Kind === 1 || f.Kind === 2) ? 'number' : 'text';
  inp.value = cur;
  inp.onchange = () => persist(f.Path, inp.value);
  return inp;
}

async function persist(path, value) {
  const r = await fetch('/api/config', {method:'POST', body: JSON.stringify({path, value})});
  if (!r.ok) { status('error: ' + await r.text()); return; }
  values[path] = value;
  status('edited (unsaved)');
}

async function save(exit) {
  const r = await fetch('/api/save', {method:'POST'});
  if (!r.ok) { status('save failed'); return; }
  status('saved — restart hermind to apply');
  if (exit) await fetch('/api/shutdown', {method:'POST'});
}

function status(s) { document.getElementById('status').textContent = s; }
boot();
```

- [ ] **Step 4: Build and manual test**

```
go build -o bin/hermind ./cmd/hermind
./bin/hermind config --web &
# open http://127.0.0.1:7777, edit a field, click Save
curl -s http://127.0.0.1:7777/api/config | head
```

- [ ] **Step 5: Commit**

```
git add cli/ui/webconfig/web/
git commit -m "feat(cli/ui/webconfig): vanilla-JS frontend"
```

---

## Task 13: Wire `config --web` and auto-open browser

**Files:**
- Modify: `hermind/cli/config.go`
- Create: `hermind/cli/ui/webconfig/openbrowser.go`

- [ ] **Step 1: Create `openbrowser.go`**

```go
package webconfig

import (
	"os/exec"
	"runtime"
)

// OpenBrowser launches the user's default browser at url. Best-effort; a
// failure is non-fatal because the URL is also printed to stderr.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
```

- [ ] **Step 2: Replace `serveWebConfig` in `cli/config.go`**

```go
func serveWebConfig(path string, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	url := "http://" + addr
	fmt.Fprintf(os.Stderr, "config editor: %s\n", url)
	go func() { _ = webconfig.OpenBrowser(url) }()
	return webconfig.Serve(path, addr)
}
```

Add imports:
```go
import (
	"fmt"
	webconfig "github.com/odysseythink/hermind/cli/ui/webconfig"
)
```

Remove the earlier placeholder return.

- [ ] **Step 3: Build and smoke test**

```
go build -o bin/hermind ./cmd/hermind
./bin/hermind config --web
```
Expected: browser opens at `http://127.0.0.1:7777`, editor renders, save works.

Test port conflict:
```
./bin/hermind config --web --port 22
# expected: "listen 127.0.0.1:22: bind: permission denied" (or similar)
```

- [ ] **Step 4: Commit**

```
git add cli/config.go cli/ui/webconfig/openbrowser.go
git commit -m "feat(cli): wire hermind config --web with auto-launch browser"
```

---

## Task 14: Documentation updates

**Files:**
- Modify: `hermind/cli/setup.go` (help text)
- Modify: root `CLAUDE.md` (if it documents commands)

- [ ] **Step 1: Update `setup.go` help text**

In `newSetupCmd`, change `Short`:
```go
Short: "Scriptable configuration writer (prefer `hermind config` for interactive use)"
```

Add to the initial println inside `runSetupInteractive`:
```go
fmt.Println("Tip: `hermind config` is the interactive TUI editor for an existing config.")
```

- [ ] **Step 2: Build**

```
go build ./...
```

- [ ] **Step 3: Commit**

```
git add cli/setup.go
git commit -m "docs(cli): point users from setup to hermind config"
```

---

## Self-Review

**1. Spec coverage:**
- Load/Save preserving comments → Task 1 ✓
- Get / Set / Remove / SetBlock → Tasks 2, 3, 4 ✓
- Schema catalog → Task 5 ✓
- TUI with section nav + field editors + list editor → Tasks 6, 7, 8 ✓
- `hermind config` subcommand → Task 9 ✓
- First-run hook → Task 10 ✓
- Web backend (all five endpoints) → Task 11 ✓
- Web frontend with embed.FS → Task 12 ✓
- `--web` wiring + auto-open browser → Task 13 ✓
- `setup` doc update → Task 14 ✓

Gaps: spec mentions "open in $EDITOR" fallback for corrupt YAML — not implemented in plan. Acceptable: spec describes it as a fallback we *can* add later; the current corruption path just surfaces a parse error through `config.Load`, which the user can fix manually. Flagging rather than inflating the plan.

**2. Placeholder scan:** one intentional placeholder in Task 9's `serveWebConfig` is explicitly marked as replaced in Task 13. Every other step contains complete code.

**3. Type consistency:** `Kind` enum values are referenced by numeric literal in `app.js` (0=String, 1=Int, 2=Float, 3=Bool, 4=Enum, 5=Secret, 6=List). Verified against `schema.go` `iota` ordering.

**4. Ambiguity check:** `providers` list section in TUI supports add via `'a'`; delete with `'d'` is referenced in the view ("[press 'a' to add, 'd' to delete]") but Task 8's implementation covers only add. Fix inline — add to Task 8's `update.go` change:

```go
		case "d":
			fields := m.fieldsInCurrentSection()
			if m.fieldIdx >= len(fields) || fields[m.fieldIdx].Kind != editor.KindList { return m, nil }
			// prompt for item key to delete — reuse fieldEditor, tagged with label prefix "del "
			ti := textinput.New(); ti.Placeholder = "name to delete"; ti.Focus()
			m.ed = &fieldEditor{field: editor.Field{Path: fields[m.fieldIdx].Path, Kind: editor.KindString, Label: "del " + fields[m.fieldIdx].Label}, input: ti}
			m.editing = true
```

And extend `commit()` in `editors.go` symmetrically:
```go
	if strings.HasPrefix(fe.field.Label, "del ") {
		name := strings.TrimSpace(fe.input.Value())
		if name == "" { return "name required" }
		if err := doc.Remove(fe.field.Path + "." + name); err != nil { return err.Error() }
		return ""
	}
```

Good — otherwise self-consistent.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-14-hermind-config-ui.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
