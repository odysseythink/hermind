# Embedding Model Configurable Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the embedding model name user-configurable via `config.yaml` and the web UI, replacing the hardcoded `"text-embedding-3-small"`.

**Architecture:** Add a top-level `embed_model` string field to `config.Config` (symmetric to `model`), register it in `config/descriptor`, and read it in `cli/engine_deps.go` when constructing the embedder. Empty value falls back to the existing default.

**Tech Stack:** Go 1.22+, testify, existing config/descriptor system

## File Structure

| File | Action | Responsibility |
|---|---|---|
| `config/config.go` | Modify | Add `EmbedModel` field to `Config`; set default in `Default()` |
| `config/config_test.go` | Modify | Verify `Default().EmbedModel` and YAML round-trip |
| `config/descriptor/embed_model.go` | Create | Register `embed_model` section for the web UI config panel |
| `config/descriptor/embed_model_test.go` | Create | Verify section registration, GroupID, field kind, default |
| `cli/engine_deps.go` | Modify | Replace hardcoded model name with config-driven value |
| `cli/engine_deps_test.go` | Modify | Add test for `resolveEmbedModel` helper |

---

### Task 1: Add `EmbedModel` to `config.Config`

**Files:**
- Modify: `config/config.go:13-14`

- [ ] **Step 1: Add field**

Insert `EmbedModel` immediately after `Model` in the `Config` struct:

```go
type Config struct {
	Model      string                    `yaml:"model"`
	EmbedModel string                    `yaml:"embed_model,omitempty"` // NEW
	Providers  map[string]ProviderConfig `yaml:"providers"`
	// ... rest unchanged
}
```

- [ ] **Step 2: Set default in `Default()`**

In `config/config.go` around line 560, add `EmbedModel` to the `Default()` return value:

```go
func Default() *Config {
	return &Config{
		Model:      "anthropic/claude-opus-4-6",
		EmbedModel: "text-embedding-3-small", // NEW
		Providers:  map[string]ProviderConfig{},
		// ... rest unchanged
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add config/config.go
git commit -m "config: add EmbedModel field to Config struct"
```

---

### Task 2: Test `EmbedModel` config behavior

**Files:**
- Modify: `config/config_test.go`

- [ ] **Step 1: Write failing test for `Default()`**

Append to `config/config_test.go`:

```go
func TestConfigDefault_EmbedModel(t *testing.T) {
	cfg := Default()
	require.Equal(t, "text-embedding-3-small", cfg.EmbedModel)
}
```

- [ ] **Step 2: Write failing test for YAML round-trip**

Append to `config/config_test.go`:

```go
func TestEmbedModelYAMLRoundTrip(t *testing.T) {
	yamlSrc := []byte("embed_model: openai/text-embedding-3-large\n")
	var cfg Config
	require.NoError(t, yaml.Unmarshal(yamlSrc, &cfg))
	require.Equal(t, "openai/text-embedding-3-large", cfg.EmbedModel)

	// Empty string should unmarshal to zero value
	var cfg2 Config
	require.NoError(t, yaml.Unmarshal([]byte("\n"), &cfg2))
	require.Equal(t, "", cfg2.EmbedModel)
}
```

- [ ] **Step 3: Run tests**

```bash
cd config && go test -v -run "TestConfigDefault_EmbedModel|TestEmbedModelYAMLRoundTrip"
```

Expected: FAIL (field doesn't exist yet — confirm before implementing).

Actually: if Task 1 is already committed, they should PASS. Run to confirm.

- [ ] **Step 4: Commit**

```bash
git add config/config_test.go
git commit -m "config: add EmbedModel default and YAML round-trip tests"
```

---

### Task 3: Register `embed_model` in config descriptor

**Files:**
- Create: `config/descriptor/embed_model.go`

- [ ] **Step 1: Create descriptor file**

Create `config/descriptor/embed_model.go`:

```go
package descriptor

func init() {
	Register(Section{
		Key:     "embed_model",
		Label:   "Embedding model",
		Summary: "Model used for text embeddings (hybrid search, topic shift detection, skill retrieval).",
		GroupID: "models",
		Shape:   ShapeScalar,
		Fields: []FieldSpec{
			{
				Name:    "embed_model",
				Label:   "Embedding model",
				Help:    "Provider-qualified id for the embedding model, e.g. openai/text-embedding-3-small. Only used when the active provider supports embeddings.",
				Kind:    FieldString,
				Default: "text-embedding-3-small",
			},
		},
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add config/descriptor/embed_model.go
git commit -m "descriptor: register embed_model section for frontend config panel"
```

---

### Task 4: Test descriptor registration

**Files:**
- Create: `config/descriptor/embed_model_test.go`

- [ ] **Step 1: Write failing test**

Create `config/descriptor/embed_model_test.go`:

```go
package descriptor

import "testing"

func TestEmbedModelSectionRegistered(t *testing.T) {
	s, ok := Get("embed_model")
	if !ok {
		t.Fatal(`Get("embed_model") returned ok=false — did embed_model.go init() register?`)
	}
	if s.GroupID != "models" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "models")
	}
	if s.Shape != ShapeScalar {
		t.Errorf("Shape = %v, want ShapeScalar", s.Shape)
	}
	if len(s.Fields) != 1 {
		t.Fatalf("len(Fields) = %d, want 1 (ShapeScalar invariant)", len(s.Fields))
	}
	f := s.Fields[0]
	if f.Name != "embed_model" {
		t.Errorf("Fields[0].Name = %q, want %q", f.Name, "embed_model")
	}
	if f.Kind != FieldString {
		t.Errorf("Fields[0].Kind = %s, want string", f.Kind)
	}
	if f.Required {
		t.Error("Fields[0].Required = true, want false (embed model has a default)")
	}
	if f.Default != "text-embedding-3-small" {
		t.Errorf("Fields[0].Default = %q, want %q", f.Default, "text-embedding-3-small")
	}
	if f.Help == "" {
		t.Error("Fields[0].Help is empty — the field needs a hint about provider-qualified format")
	}
}
```

- [ ] **Step 2: Run test**

```bash
cd config/descriptor && go test -v -run TestEmbedModelSectionRegistered
```

Expected: PASS (Task 3 already committed).

- [ ] **Step 3: Commit**

```bash
git add config/descriptor/embed_model_test.go
git commit -m "descriptor: add embed_model section registration test"
```

---

### Task 5: Wire config into embedder construction

**Files:**
- Modify: `cli/engine_deps.go:175-184`

- [ ] **Step 1: Extract `resolveEmbedModel` helper**

Add this unexported function near the top of `cli/engine_deps.go` (after imports or before `BuildEngineDeps`):

```go
// resolveEmbedModel returns the configured embedding model name,
// falling back to the hardcoded default when empty.
func resolveEmbedModel(cfg *config.Config) string {
	if cfg.EmbedModel != "" {
		return cfg.EmbedModel
	}
	return "text-embedding-3-small"
}
```

- [ ] **Step 2: Replace hardcoded string in `BuildEngineDeps`**

Change lines 175-184 in `cli/engine_deps.go` from:

```go
var emb embedding.Embedder
if p != nil {
	if ec, ok := p.(provider.EmbedCapable); ok {
		emb = embedding.NewProviderEmbedder(ec, "text-embedding-3-small")
	}
	// TODO: pantheon core.LanguageModel does not expose embedding yet;
	// embedding-dependent features are disabled when the provider is not
	// EmbedCapable.
}
```

To:

```go
var emb embedding.Embedder
if p != nil {
	if ec, ok := p.(provider.EmbedCapable); ok {
		emb = embedding.NewProviderEmbedder(ec, resolveEmbedModel(app.Config))
	}
	// TODO: pantheon core.LanguageModel does not expose embedding yet;
	// embedding-dependent features are disabled when the provider is not
	// EmbedCapable.
}
```

- [ ] **Step 3: Commit**

```bash
git add cli/engine_deps.go
git commit -m "cli: read embed_model from config instead of hardcoded default"
```

---

### Task 6: Test `resolveEmbedModel` helper

**Files:**
- Modify: `cli/engine_deps_test.go`

- [ ] **Step 1: Write test**

Append to `cli/engine_deps_test.go`:

```go
func TestResolveEmbedModel(t *testing.T) {
	// Custom value is used when set
	cfg := &config.Config{EmbedModel: "openai/text-embedding-3-large"}
	require.Equal(t, "openai/text-embedding-3-large", resolveEmbedModel(cfg))

	// Empty string falls back to default
	cfg2 := &config.Config{}
	require.Equal(t, "text-embedding-3-small", resolveEmbedModel(cfg2))

	// Whitespace-only is NOT treated as empty — passes through as-is
	cfg3 := &config.Config{EmbedModel: "   "}
	require.Equal(t, "   ", resolveEmbedModel(cfg3))
}
```

- [ ] **Step 2: Run test**

```bash
cd cli && go test -v -run TestResolveEmbedModel
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add cli/engine_deps_test.go
git commit -m "cli: add resolveEmbedModel unit tests"
```

---

### Task 7: Run full test suite

- [ ] **Step 1: Run all modified packages**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind

go test ./config/... ./config/descriptor/... ./cli/... -count=1
```

Expected: all PASS.

- [ ] **Step 2: Run existing engine_deps tests to ensure no regression**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind

go test ./cli/... -run TestBuildEngineDeps -count=1 -v
```

Expected: `TestBuildEngineDeps_Smoke` and `TestBuildEngineDeps_AuxFallsBackToMainProvider` both PASS.

- [ ] **Step 3: Commit (if any fixes needed)**

If any fixes were required, commit them. Otherwise, no additional commit.

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] Top-level `embed_model` field in `Config` → Task 1
- [x] Default value `"text-embedding-3-small"` → Task 1, Task 2
- [x] Frontend descriptor registration → Task 3, Task 4
- [x] Config-driven embedder construction → Task 5
- [x] Empty-string fallback → Task 5, Task 6
- [x] Graceful degradation (unchanged) → implicit in existing code

**2. Placeholder scan:**
- [x] No "TBD", "TODO", "implement later"
- [x] All test code is concrete
- [x] All file paths are exact
- [x] All commands have expected outputs

**3. Type consistency:**
- [x] `EmbedModel string` in `Config` matches `cfg.EmbedModel` in `resolveEmbedModel`
- [x] `yaml:"embed_model,omitempty"` matches descriptor key `"embed_model"`
- [x] Default value `"text-embedding-3-small"` is identical everywhere
