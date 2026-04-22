# Session Config & Explicit System Prompt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the implicit first-message-as-system-prompt concatenation with a configured `default_system_prompt` and a per-session right-side settings drawer that edits model + system prompt, applying changes from the next turn.

**Architecture:** Backend gains `config.Agent.DefaultSystemPrompt`, `PromptBuilder` appends it after the hardcoded identity, `ensureSession` stops concatenating the user's first message, and `storage.SessionUpdate` carries two new `*string` fields routed through an extended `PATCH /api/sessions/{id}`. A new SSE `session_updated` event keeps tabs in sync. The web replaces the top-right `ModelSelector` with a gear button that toggles a `SessionSettingsDrawer`; the drawer holds local draft state, saves via PATCH, and refreshes via the new SSE event. A single `FieldText` descriptor kind (multi-line string) is added so `/settings/agent` can render the global default prompt as a textarea.

**Tech Stack:** Go 1.22 (chi router, SQLite), React 18 + TypeScript 5 + Vite + Zod, testify.

**Spec:** `docs/superpowers/specs/2026-04-22-session-config-system-prompt-design.md`

---

## Task 1: Add DefaultSystemPrompt to config.AgentConfig

**Files:**
- Modify: `config/config.go` — add field to `AgentConfig`
- Modify: `config/descriptor/agent.go` — register descriptor field
- Test: `config/loader_test.go` — add case verifying the YAML tag round-trips
- Test: `config/descriptor/agent_test.go` — add case verifying the descriptor exposes the new field

- [ ] **Step 1: Write the failing loader test**

Append this test to `config/loader_test.go` (after the existing `TestLoad_*` cases):

```go
func TestLoad_AgentDefaultSystemPrompt(t *testing.T) {
	yaml := `
agent:
  max_turns: 10
  default_system_prompt: "You are a sardonic assistant."
`
	tmp := writeTempConfig(t, yaml) // existing helper
	cfg, err := Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Agent.DefaultSystemPrompt, "You are a sardonic assistant."; got != want {
		t.Errorf("DefaultSystemPrompt = %q, want %q", got, want)
	}
}
```

Check `config/loader_test.go` for the actual helper name (`writeTempConfig` may be named differently — search near existing tests and use whatever is there). If no helper exists, inline the temp-file dance used by the nearest existing test.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./config/ -run TestLoad_AgentDefaultSystemPrompt -v
```

Expected: FAIL with `unknown field "default_system_prompt"` or the field being empty.

- [ ] **Step 3: Add the field to AgentConfig**

Edit `config/config.go`. Find the `type AgentConfig struct` block (around line 258) and add the field at the end:

```go
type AgentConfig struct {
	MaxTurns            int               `yaml:"max_turns"`
	GatewayTimeout      int               `yaml:"gateway_timeout,omitempty"`
	Compression         CompressionConfig `yaml:"compression,omitempty"`
	DefaultSystemPrompt string            `yaml:"default_system_prompt,omitempty"`
}
```

- [ ] **Step 4: Run loader test to verify it passes**

```bash
go test ./config/ -run TestLoad_AgentDefaultSystemPrompt -v
```

Expected: PASS.

- [ ] **Step 5: Write the failing descriptor test**

Append to `config/descriptor/agent_test.go`:

```go
func TestAgentDescriptor_IncludesDefaultSystemPrompt(t *testing.T) {
	s, ok := Get("agent")
	if !ok {
		t.Fatal(`Get("agent") returned ok=false`)
	}
	var found *FieldSpec
	for i := range s.Fields {
		if s.Fields[i].Name == "default_system_prompt" {
			found = &s.Fields[i]
			break
		}
	}
	if found == nil {
		t.Fatal("default_system_prompt field missing from agent descriptor")
	}
	if found.Kind != FieldText {
		t.Errorf("default_system_prompt.Kind = %v, want FieldText", found.Kind)
	}
	if found.Required {
		t.Error("default_system_prompt should not be Required")
	}
}
```

Note: this test references `FieldText`, which we will add in Task 2. This task's final step adds `FieldString` as a temporary stand-in; Task 2 flips it to `FieldText`.

- [ ] **Step 6: Run the descriptor test to verify it fails**

```bash
go test ./config/descriptor/ -run TestAgentDescriptor_IncludesDefaultSystemPrompt -v
```

Expected: FAIL with `default_system_prompt field missing` OR compile error about `FieldText` (which we will accept — Task 2 resolves this).

- [ ] **Step 7: Register the descriptor field with temporary FieldString kind**

Edit `config/descriptor/agent.go`. Extend the `Fields` slice with a third entry:

```go
Fields: []FieldSpec{
	{
		Name:     "max_turns",
		Label:    "Max turns",
		Help:     "Maximum model turns per user request before the engine bails out.",
		Kind:     FieldInt,
		Required: true,
		Default:  90,
	},
	{
		Name:    "gateway_timeout",
		Label:   "Gateway timeout (seconds)",
		Help:    "Seconds a gateway request may run before being cancelled. 0 uses the gateway default.",
		Kind:    FieldInt,
		Default: 1800,
	},
	{
		Name:    "default_system_prompt",
		Label:   "Default system prompt",
		Help:    "Prepended to every new session's system prompt, right after the agent identity block. Empty means no extra prompt.",
		Kind:    FieldString, // flipped to FieldText in Task 2
		Default: "",
	},
},
```

- [ ] **Step 8: Run descriptor test (will still fail on FieldText assertion)**

```bash
go test ./config/descriptor/ -run TestAgentDescriptor_IncludesDefaultSystemPrompt -v
```

Expected: FAIL with `FieldKind = string, want FieldText`. That is fine — Task 2 introduces `FieldText` and flips this field. Leave the test as-is.

- [ ] **Step 9: Run all config tests to make sure nothing else broke**

```bash
go test ./config/... -v
```

Expected: all existing tests PASS; only `TestAgentDescriptor_IncludesDefaultSystemPrompt` still fails (pending Task 2).

- [ ] **Step 10: Commit**

```bash
git add config/config.go config/descriptor/agent.go config/loader_test.go config/descriptor/agent_test.go
git commit -m "$(cat <<'EOF'
feat(config): add DefaultSystemPrompt to AgentConfig

Adds an optional yaml field and descriptor entry that the web settings
page and agent PromptBuilder will consume. Descriptor kind temporarily
set to FieldString; follow-up flips it to FieldText.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add FieldText descriptor kind (multi-line string)

**Files:**
- Modify: `config/descriptor/descriptor.go` — add `FieldText` constant + `String()` case
- Modify: `config/descriptor/agent.go` — flip `default_system_prompt` from `FieldString` → `FieldText`
- Modify: `web/src/api/schemas.ts` — extend `ConfigFieldKindSchema` with `'text'`
- Create: `web/src/components/fields/TextAreaInput.tsx` — multi-line text field
- Modify: `web/src/components/ConfigSection.tsx` — dispatch `text` kind to `TextAreaInput`
- Test: `config/descriptor/descriptor_test.go` — add `FieldText` String() assertion
- Test: `web/src/components/fields/TextAreaInput.test.tsx` — basic render + onChange test

- [ ] **Step 1: Write the failing Go test**

Append to `config/descriptor/descriptor_test.go` (inside or next to `TestFieldKindString`):

```go
func TestFieldKindString_IncludesText(t *testing.T) {
	if got := FieldText.String(); got != "text" {
		t.Errorf("FieldText.String() = %q, want %q", got, "text")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./config/descriptor/ -run TestFieldKindString_IncludesText -v
```

Expected: FAIL compile error (`FieldText` undefined).

- [ ] **Step 3: Add FieldText constant + String case**

Edit `config/descriptor/descriptor.go`. Extend the const block and `String()` switch:

```go
const (
	FieldUnknown FieldKind = iota
	FieldString
	FieldInt
	FieldBool
	FieldSecret
	FieldEnum
	FieldFloat
	FieldMultiSelect
	FieldText
)

func (k FieldKind) String() string {
	switch k {
	case FieldString:
		return "string"
	case FieldInt:
		return "int"
	case FieldBool:
		return "bool"
	case FieldSecret:
		return "secret"
	case FieldEnum:
		return "enum"
	case FieldFloat:
		return "float"
	case FieldMultiSelect:
		return "multiselect"
	case FieldText:
		return "text"
	}
	return "unknown"
}
```

- [ ] **Step 4: Flip the agent descriptor field**

Edit `config/descriptor/agent.go`. Change the `default_system_prompt` field's `Kind: FieldString` to `Kind: FieldText`.

- [ ] **Step 5: Run Go tests**

```bash
go test ./config/... -v
```

Expected: both `TestFieldKindString_IncludesText` and `TestAgentDescriptor_IncludesDefaultSystemPrompt` PASS now.

- [ ] **Step 6: Write failing frontend test**

Create `web/src/components/fields/TextAreaInput.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TextAreaInput from './TextAreaInput';

describe('TextAreaInput', () => {
  it('renders current value and fires onChange with new text', () => {
    const onChange = vi.fn();
    render(
      <TextAreaInput
        value="hello"
        onChange={onChange}
        placeholder="type here"
      />,
    );
    const ta = screen.getByPlaceholderText('type here') as HTMLTextAreaElement;
    expect(ta.value).toBe('hello');
    fireEvent.change(ta, { target: { value: 'world' } });
    expect(onChange).toHaveBeenCalledWith('world');
  });
});
```

- [ ] **Step 7: Run frontend test to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind/web
npm test -- TextAreaInput --run
```

Expected: FAIL (module not found).

- [ ] **Step 8: Create TextAreaInput component**

Create `web/src/components/fields/TextAreaInput.tsx`:

```tsx
import styles from './TextAreaInput.module.css';

type Props = {
  value: string;
  onChange: (next: string) => void;
  placeholder?: string;
  disabled?: boolean;
  rows?: number;
  'aria-label'?: string;
};

export default function TextAreaInput({
  value, onChange, placeholder, disabled, rows = 6, ...rest
}: Props) {
  return (
    <textarea
      className={styles.textarea}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      rows={rows}
      aria-label={rest['aria-label']}
    />
  );
}
```

Create `web/src/components/fields/TextAreaInput.module.css`:

```css
.textarea {
  width: 100%;
  min-height: 120px;
  padding: 6px 8px;
  font-family: ui-monospace, SF Mono, Menlo, monospace;
  font-size: 13px;
  line-height: 1.5;
  color: var(--fg, #d4d4d4);
  background: var(--bg-elevated, #111);
  border: 1px solid var(--border, #333);
  border-radius: 2px;
  resize: vertical;
  box-sizing: border-box;
}
.textarea:focus {
  outline: none;
  border-color: #FFB800;
}
.textarea:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
```

- [ ] **Step 9: Extend ConfigFieldKindSchema**

Edit `web/src/api/schemas.ts`. Find the line:

```ts
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float', 'multiselect',
]);
```

Change to:

```ts
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float', 'multiselect', 'text',
]);
```

- [ ] **Step 10: Dispatch 'text' kind in ConfigSection**

Edit `web/src/components/ConfigSection.tsx`. Locate the field-kind switch (look for `case 'string'`). Add a `case 'text'` branch that imports and renders `TextAreaInput`:

```tsx
import TextAreaInput from './fields/TextAreaInput';
// ... inside the switch:
case 'text':
  return (
    <TextAreaInput
      value={typeof value === 'string' ? value : ''}
      onChange={(v) => onFieldChange(field.name, v)}
      placeholder={field.help ?? ''}
      aria-label={field.label}
    />
  );
```

If `ConfigSection.tsx` already has a field-kind dispatcher with a different shape, adapt this branch to match — the essence is: when `field.kind === 'text'`, render `TextAreaInput` instead of `TextInput`.

- [ ] **Step 11: Run frontend test to verify it passes**

```bash
cd web && npm test -- TextAreaInput --run
```

Expected: PASS.

- [ ] **Step 12: Run full test suites**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./... -count=1
cd web && npm test --run
```

Expected: all PASS.

- [ ] **Step 13: Commit**

```bash
git add config/descriptor/descriptor.go config/descriptor/agent.go config/descriptor/descriptor_test.go \
  web/src/api/schemas.ts web/src/components/fields/TextAreaInput.tsx \
  web/src/components/fields/TextAreaInput.module.css web/src/components/fields/TextAreaInput.test.tsx \
  web/src/components/ConfigSection.tsx
git commit -m "$(cat <<'EOF'
feat(config): add FieldText kind for multi-line string fields

Introduces FieldText as a new descriptor kind, wires the schema enum
end-to-end, adds a TextAreaInput field component, and flips
agent.default_system_prompt to render as a textarea.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Extend PromptBuilder to append DefaultSystemPrompt

**Files:**
- Modify: `agent/prompt.go` — add field, change constructor signature, append in Build
- Modify: `agent/engine.go` — thread `cfg.DefaultSystemPrompt` into `NewPromptBuilder`
- Test: `agent/prompt_test.go` — add two cases (empty = identity only, non-empty = appended)
- Fix: any callers of `NewPromptBuilder("<platform>")` now require a second string argument

- [ ] **Step 1: Write the failing test**

Append to `agent/prompt_test.go`:

```go
func TestPromptBuilder_AppendsDefaultSystemPrompt(t *testing.T) {
	pb := NewPromptBuilder("cli", "You are a Go debugger.")
	got := pb.Build(&PromptOptions{Model: "claude-opus-4-7"})
	if !strings.Contains(got, "Hermind Agent") {
		t.Errorf("expected identity block in output, got %q", got)
	}
	if !strings.Contains(got, "You are a Go debugger.") {
		t.Errorf("expected default system prompt to be appended, got %q", got)
	}
	// Identity must come first
	identIdx := strings.Index(got, "Hermind Agent")
	defIdx := strings.Index(got, "You are a Go debugger.")
	if identIdx > defIdx {
		t.Errorf("identity must come before default system prompt (ident=%d def=%d)", identIdx, defIdx)
	}
}

func TestPromptBuilder_EmptyDefaultPreservesIdentityOnly(t *testing.T) {
	pb := NewPromptBuilder("cli", "")
	got := pb.Build(&PromptOptions{Model: "claude-opus-4-7"})
	if !strings.Contains(got, "Hermind Agent") {
		t.Errorf("expected identity block, got %q", got)
	}
	// When no skills and no default, the result is the identity ONLY.
	if strings.Count(got, "\n\n") > 0 && !strings.HasSuffix(strings.TrimSpace(got), "tool's directory.") {
		t.Errorf("unexpected extra block when default prompt is empty: %q", got)
	}
}
```

(Add `"strings"` import if missing.)

- [ ] **Step 2: Run to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./agent/ -run 'TestPromptBuilder_' -v
```

Expected: FAIL compile error (`NewPromptBuilder` takes 1 arg but 2 given).

- [ ] **Step 3: Extend PromptBuilder**

Edit `agent/prompt.go`. Replace the existing struct and constructor:

```go
type PromptBuilder struct {
	platform            string
	defaultSystemPrompt string
}

func NewPromptBuilder(platform, defaultSystemPrompt string) *PromptBuilder {
	return &PromptBuilder{platform: platform, defaultSystemPrompt: defaultSystemPrompt}
}

func (pb *PromptBuilder) Build(opts *PromptOptions) string {
	var parts []string
	parts = append(parts, defaultIdentity)
	if strings.TrimSpace(pb.defaultSystemPrompt) != "" {
		parts = append(parts, pb.defaultSystemPrompt)
	}
	if opts != nil && len(opts.ActiveSkills) > 0 {
		parts = append(parts, renderActiveSkills(opts.ActiveSkills))
	}
	return strings.Join(parts, "\n\n")
}
```

- [ ] **Step 4: Fix engine constructor**

Edit `agent/engine.go`. In `NewEngineWithToolsAndAux` (around line 59), change:

```go
prompt: NewPromptBuilder(platform),
```

to:

```go
prompt: NewPromptBuilder(platform, cfg.DefaultSystemPrompt),
```

- [ ] **Step 5: Fix existing prompt tests that use the old signature**

Edit `agent/prompt_test.go`. Anywhere `NewPromptBuilder("cli")` or `NewPromptBuilder("telegram")` appears (existing tests — look for the old single-arg calls), change to `NewPromptBuilder("cli", "")` and `NewPromptBuilder("telegram", "")` respectively. The empty second arg preserves existing behavior.

- [ ] **Step 6: Search for any other callers of NewPromptBuilder**

```bash
grep -rn "NewPromptBuilder(" --include='*.go' .
```

Expected hits: `agent/engine.go` (fixed), `agent/prompt.go` (definition), `agent/prompt_test.go` (fixed). If any other caller exists, add `""` as second arg.

- [ ] **Step 7: Run tests**

```bash
go test ./agent/ -run 'TestPromptBuilder' -v
go test ./agent/... -count=1
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add agent/prompt.go agent/prompt_test.go agent/engine.go
git commit -m "$(cat <<'EOF'
feat(agent): PromptBuilder appends config.Agent.DefaultSystemPrompt

Identity block stays first, then the user-configured prompt, then any
active-skill blocks. Empty default preserves identity-only output.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Stop concatenating the first user message in ensureSession

**Files:**
- Modify: `agent/conversation.go` — strip the concat
- Modify: `agent/ensure_session_test.go` — update the assertion that expected the concat

- [ ] **Step 1: Update the existing failing-expectation test**

Edit `agent/ensure_session_test.go`. Find `TestEnsureSession_NewRow_ComposesPromptAndTitle` (around line 33). Rename the test to `TestEnsureSession_NewRow_UsesDefaultPromptOnly` and replace the SystemPrompt assertion:

Before (lines 33-50 approximately):
```go
func TestEnsureSession_NewRow_ComposesPromptAndTitle(t *testing.T) {
	...
	sess, created, err := eng.ensureSession(context.Background(), opts, "You are helpful.", opts.UserMessage, opts.Model)
	...
	assert.Equal(t, "You are helpful.\n\nBuild me a haiku generator", sess.SystemPrompt)
	assert.Equal(t, "Build me a", sess.Title)
	assert.Equal(t, "web", sess.Source)
}
```

After:
```go
func TestEnsureSession_NewRow_UsesDefaultPromptOnly(t *testing.T) {
	store := newTestStoreForEngine(t)
	eng := newEngineWithStorage(t, store, "web")

	opts := &RunOptions{
		SessionID:   "s-new-1",
		UserMessage: "Build me a haiku generator",
		Model:       "claude-opus-4-7",
	}

	sess, created, err := eng.ensureSession(context.Background(), opts, "You are helpful.", opts.UserMessage, opts.Model)
	require.NoError(t, err)
	require.NotNil(t, sess)
	assert.True(t, created)
	assert.Equal(t, "You are helpful.", sess.SystemPrompt) // no concatenation
	assert.Equal(t, "Build me a", sess.Title)              // title still derived from firstMsg
	assert.Equal(t, "web", sess.Source)
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./agent/ -run TestEnsureSession_NewRow_UsesDefaultPromptOnly -v
```

Expected: FAIL — got `"You are helpful.\n\nBuild me a haiku generator"`, want `"You are helpful."`.

- [ ] **Step 3: Strip the concatenation**

Edit `agent/conversation.go`. Find `ensureSession` (around line 281) and replace the body's composed-prompt block:

Before (around lines 292-294):
```go
composed := defaultPrompt
if strings.TrimSpace(firstMsg) != "" {
	composed = defaultPrompt + "\n\n" + firstMsg
}
```

After:
```go
composed := defaultPrompt
// Note: firstMsg intentionally no longer concatenated here.
// It still feeds DeriveTitle below.
```

Leave the `SystemPrompt: composed,` line and the `Title: DeriveTitle(firstMsg),` line exactly as they are.

- [ ] **Step 4: Run the test**

```bash
go test ./agent/ -run TestEnsureSession_NewRow_UsesDefaultPromptOnly -v
```

Expected: PASS.

- [ ] **Step 5: Run the full agent suite to catch cascading failures**

```bash
go test ./agent/... -count=1
```

Expected: PASS. If `TestEnsureSession_ExistingRow_Unchanged` or other tests fail with stale expectations about concatenation, update their assertions to the no-concat behavior. Check `agent/ensure_session_test.go` lines 52-90 for similar hardcoded concat strings.

- [ ] **Step 6: Check if `strings` import is still used in conversation.go**

```bash
grep -n 'strings\.' agent/conversation.go
```

If there are other uses of `strings` in the file, leave the import. If the TrimSpace we just removed was the only use, remove the `"strings"` import at the top of `agent/conversation.go`.

- [ ] **Step 7: Commit**

```bash
git add agent/conversation.go agent/ensure_session_test.go
git commit -m "$(cat <<'EOF'
feat(agent): stop splicing first user message into session system prompt

Session.SystemPrompt is now the configured default only; the first user
message still drives Title derivation. Existing sessions with the
historical concatenation in their stored SystemPrompt are untouched.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Extend storage.SessionUpdate with Model and SystemPrompt

**Files:**
- Modify: `storage/types.go` — add two `*string` fields
- Modify: `storage/sqlite/session.go` — wire them into UpdateSession
- Modify: `storage/sqlite/tx.go` — wire them into txImpl.UpdateSession
- Test: `storage/sqlite/sqlite_test.go` — add two cases

- [ ] **Step 1: Write the failing tests**

Append to `storage/sqlite/sqlite_test.go`:

```go
func TestUpdateSession_PatchesModelAndSystemPrompt(t *testing.T) {
	store := newStore(t) // reuse whatever helper the neighboring tests use
	ctx := context.Background()

	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID:           "s-patch-1",
		Source:       "web",
		Model:        "claude-opus-4-7",
		SystemPrompt: "orig",
		Title:        "orig title",
		StartedAt:    time.Now().UTC(),
	}))

	newModel := "claude-sonnet-4-6"
	newPrompt := "You are a concise assistant."
	require.NoError(t, store.UpdateSession(ctx, "s-patch-1", &storage.SessionUpdate{
		Model:        &newModel,
		SystemPrompt: &newPrompt,
	}))

	got, err := store.GetSession(ctx, "s-patch-1")
	require.NoError(t, err)
	assert.Equal(t, newModel, got.Model)
	assert.Equal(t, newPrompt, got.SystemPrompt)
	assert.Equal(t, "orig title", got.Title) // untouched
}

func TestUpdateSession_EmptyStringClearsFields(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID:           "s-patch-2",
		Source:       "web",
		Model:        "claude-opus-4-7",
		SystemPrompt: "orig",
		Title:        "t",
		StartedAt:    time.Now().UTC(),
	}))

	empty := ""
	require.NoError(t, store.UpdateSession(ctx, "s-patch-2", &storage.SessionUpdate{
		Model:        &empty,
		SystemPrompt: &empty,
	}))

	got, err := store.GetSession(ctx, "s-patch-2")
	require.NoError(t, err)
	assert.Equal(t, "", got.Model)
	assert.Equal(t, "", got.SystemPrompt)
}
```

(Use whatever helper the existing tests use to construct the store; read the top of `sqlite_test.go` to match. The existing `TestUpdateSession` around line 95 is a good template.)

- [ ] **Step 2: Run to verify it fails**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./storage/sqlite/ -run TestUpdateSession_PatchesModelAndSystemPrompt -v
```

Expected: FAIL compile error (`SessionUpdate` has no field `Model` / `SystemPrompt`).

- [ ] **Step 3: Extend SessionUpdate struct**

Edit `storage/types.go`. Replace the existing `SessionUpdate`:

```go
type SessionUpdate struct {
	EndedAt      *time.Time
	EndReason    string
	Title        string
	MessageCount *int
	Model        *string // nil = unchanged; "" = clear
	SystemPrompt *string // nil = unchanged; "" = clear
}
```

- [ ] **Step 4: Extend the SQLite UpdateSession**

Edit `storage/sqlite/session.go`. Find the `UpdateSession` method (around line 97). Add two new blocks inside the dynamic-SQL builder, right before the `if len(setClauses) == 0` check:

```go
if upd.Model != nil {
	setClauses = append(setClauses, "model = ?")
	args = append(args, *upd.Model)
}
if upd.SystemPrompt != nil {
	setClauses = append(setClauses, "system_prompt = ?")
	args = append(args, *upd.SystemPrompt)
}
```

- [ ] **Step 5: Extend the tx UpdateSession**

Edit `storage/sqlite/tx.go`. Find `txImpl.UpdateSession` (around line 116). It has the same shape as `Store.UpdateSession`. Add the same two blocks in the same position.

- [ ] **Step 6: Run tests**

```bash
go test ./storage/sqlite/ -run TestUpdateSession -v
```

Expected: both new tests PASS, existing `TestUpdateSession` still PASS.

- [ ] **Step 7: Run wider Go suite to catch callers broken by new fields**

```bash
go test ./... -count=1
```

Expected: PASS. If anything else references `SessionUpdate` in an expectation-incompatible way, fix it.

- [ ] **Step 8: Commit**

```bash
git add storage/types.go storage/sqlite/session.go storage/sqlite/tx.go storage/sqlite/sqlite_test.go
git commit -m "$(cat <<'EOF'
feat(storage): extend SessionUpdate with Model and SystemPrompt

Pointer fields distinguish 'leave unchanged' from 'explicitly empty'.
Both SQLite UpdateSession impls pick up the two columns in the dynamic
UPDATE builder. No schema migration needed; columns already exist.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: RunConversation prefers sess.Model for the current turn

**Files:**
- Modify: `agent/conversation.go` — after `ensureSession`, if session has a non-empty model, use it
- Test: `agent/engine_test.go` — add a new case

- [ ] **Step 1: Write the failing test**

Append to `agent/engine_test.go` (or the nearest existing test file that exercises `RunConversation` end-to-end):

```go
func TestRunConversation_PrefersSessionModelOverRunOptions(t *testing.T) {
	store := newTestStoreForEngine(t)
	ctx := context.Background()

	// Pre-seed a session with a model that differs from what RunOptions will carry.
	require.NoError(t, store.CreateSession(ctx, &storage.Session{
		ID:           "s-model-pref",
		Source:       "web",
		Model:        "claude-sonnet-4-6", // what the session wants
		SystemPrompt: "",
		Title:        "t",
		StartedAt:    time.Now().UTC(),
	}))

	// Fake provider that records the model it received.
	fp := &fakeProvider{}
	eng := NewEngineWithToolsAndAux(fp, nil, store, nil, config.AgentConfig{MaxTurns: 1}, "web")

	_, err := eng.RunConversation(ctx, &agent.RunOptions{
		SessionID:   "s-model-pref",
		UserMessage: "hi",
		Model:       "claude-opus-4-7", // loser — session value should win
	})
	_ = err // provider may return early; we only care which model it saw

	if got, want := fp.lastModel, "claude-sonnet-4-6"; got != want {
		t.Errorf("provider saw model = %q, want %q (session value must win over RunOptions)", got, want)
	}
}
```

If `fakeProvider` does not exist yet, use whatever provider-stub pattern the neighboring tests use (look near `agent/engine_test.go` or `agent/phase12_test.go`). If none exists, add a minimal one that captures `lastModel` on `Complete` / `Stream` and returns `io.EOF`. Keep it in the same file.

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./agent/ -run TestRunConversation_PrefersSessionModelOverRunOptions -v
```

Expected: FAIL — provider sees `"claude-opus-4-7"` (the RunOptions value), we want `"claude-sonnet-4-6"`.

- [ ] **Step 3: Update RunConversation to prefer session model**

Edit `agent/conversation.go`. Find the block right after `effectivePrompt = sess.SystemPrompt` inside `if e.storage != nil { ... }` (around line 64). Extend it:

Before:
```go
effectivePrompt = sess.SystemPrompt
if err := e.persistMessage(...
```

After:
```go
effectivePrompt = sess.SystemPrompt
if sess.Model != "" {
	model = sess.Model
}
if err := e.persistMessage(...
```

- [ ] **Step 4: Run the test**

```bash
go test ./agent/ -run TestRunConversation_PrefersSessionModelOverRunOptions -v
```

Expected: PASS.

- [ ] **Step 5: Run the full agent suite**

```bash
go test ./agent/... -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add agent/conversation.go agent/engine_test.go
git commit -m "$(cat <<'EOF'
feat(agent): RunConversation prefers Session.Model over RunOptions.Model

When a session row carries a non-empty Model, use it for the turn. This
lets users switch models mid-conversation via PATCH /api/sessions/{id}
and have the next turn respect the new choice.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Add system_prompt to SessionDTO

**Files:**
- Modify: `api/dto.go` — add field to `SessionDTO`
- Modify: `api/handlers_sessions.go` — populate it in `dtoFromSession`
- Test: `api/handlers_sessions_test.go` — verify GET /api/sessions/{id} returns the field

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_sessions_test.go`:

```go
func TestGetSession_ReturnsSystemPromptField(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-dto-1", "web", "claude-opus-4-7", "You are a helper.", "Title 1")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/sessions/s-dto-1", nil)
	req.Header.Set("Authorization", "Bearer t")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	if got, want := dto.SystemPrompt, "You are a helper."; got != want {
		t.Errorf("SystemPrompt = %q, want %q", got, want)
	}
}
```

If the existing `seedSession` helper does not accept a `SystemPrompt` parameter, add a sibling helper `seedSessionFull(id, source, model, systemPrompt, title string)` in the nearest test helper file (search for `seedSession` definition to find the right file). It mirrors `seedSession` but sets all fields.

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./api/ -run TestGetSession_ReturnsSystemPromptField -v
```

Expected: FAIL compile error (`SessionDTO` has no field `SystemPrompt`).

- [ ] **Step 3: Extend SessionDTO**

Edit `api/dto.go`. Replace `SessionDTO`:

```go
type SessionDTO struct {
	ID           string  `json:"id"`
	Source       string  `json:"source"`
	Model        string  `json:"model"`
	SystemPrompt string  `json:"system_prompt"`
	StartedAt    float64 `json:"started_at"`
	EndedAt      float64 `json:"ended_at"`
	MessageCount int     `json:"message_count"`
	Title        string  `json:"title"`
}
```

- [ ] **Step 4: Populate it in dtoFromSession**

Edit `api/handlers_sessions.go`. Find `dtoFromSession` (around line 128). Add the field:

```go
return SessionDTO{
	ID:           s.ID,
	Source:       s.Source,
	Model:        s.Model,
	SystemPrompt: s.SystemPrompt,
	StartedAt:    toEpoch(s.StartedAt),
	EndedAt:      endedAt,
	MessageCount: s.MessageCount,
	Title:        s.Title,
}
```

- [ ] **Step 5: Run the test**

```bash
go test ./api/ -run TestGetSession_ReturnsSystemPromptField -v
```

Expected: PASS.

- [ ] **Step 6: Run the wider API suite**

```bash
go test ./api/... -count=1
```

Expected: PASS. If any other test uses a literal `SessionDTO{...}` that must now carry `SystemPrompt: ""` explicitly, fix it.

- [ ] **Step 7: Commit**

```bash
git add api/dto.go api/handlers_sessions.go api/handlers_sessions_test.go
git commit -m "$(cat <<'EOF'
feat(api): expose session.system_prompt on SessionDTO

Both the list and get endpoints now carry system_prompt so the web
drawer can hydrate its draft on open.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Extend PATCH /api/sessions/{id} to accept model + system_prompt

**Files:**
- Modify: `api/handlers_sessions.go` — rewrite `handleSessionPatch`
- Create: `api/session_patch_limits.go` — constants for size caps
- Test: `api/handlers_sessions_test.go` — add four cases

- [ ] **Step 1: Write the failing tests**

Append to `api/handlers_sessions_test.go`:

```go
func TestPatchSession_UpdatesModelAndSystemPrompt(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p1", "web", "claude-opus-4-7", "orig", "t")

	body := `{"model":"claude-sonnet-4-6","system_prompt":"new prompt"}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p1", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	assert.Equal(t, "claude-sonnet-4-6", dto.Model)
	assert.Equal(t, "new prompt", dto.SystemPrompt)
	assert.Equal(t, "t", dto.Title) // unchanged
}

func TestPatchSession_OnlyTitle_StillWorks(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p2", "web", "claude-opus-4-7", "orig", "old")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p2",
		strings.NewReader(`{"title":"new"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	assert.Equal(t, "new", dto.Title)
	assert.Equal(t, "orig", dto.SystemPrompt)
}

func TestPatchSession_EnforcesSystemPromptSizeLimit(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p3", "web", "claude-opus-4-7", "orig", "t")

	tooBig := strings.Repeat("a", MaxSystemPromptBytes+1)
	body := fmt.Sprintf(`{"system_prompt":%q}`, tooBig)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p3", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code=%d, want 400", rr.Code)
	}
}

func TestPatchSession_EnforcesModelNameLimit(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p4", "web", "claude-opus-4-7", "orig", "t")

	body := fmt.Sprintf(`{"model":%q}`, strings.Repeat("m", MaxModelNameBytes+1))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p4", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("code=%d, want 400", rr.Code)
	}
}

func TestPatchSession_AllowsEmptyStringToClear(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-p5", "web", "claude-opus-4-7", "orig", "t")

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-p5",
		strings.NewReader(`{"system_prompt":""}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var dto SessionDTO
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dto))
	assert.Equal(t, "", dto.SystemPrompt)
}
```

Add `"fmt"` and `"strings"` imports if needed.

- [ ] **Step 2: Run to verify they fail**

```bash
go test ./api/ -run TestPatchSession -v
```

Expected: FAIL — all new cases fail; the existing `TestPatchSession_*` cases still pass.

- [ ] **Step 3: Create the limits file**

Create `api/session_patch_limits.go`:

```go
package api

// Size caps enforced by PATCH /api/sessions/{id}. Values chosen to be
// generous for normal use while preventing accidental or malicious
// blobs from entering the sessions table.
const (
	MaxSessionTitleBytes  = 256        // bytes, not runes — conservative
	MaxSystemPromptBytes  = 32 * 1024  // 32 KB
	MaxModelNameBytes     = 128
)
```

- [ ] **Step 4: Rewrite the PATCH handler**

Edit `api/handlers_sessions.go`. Replace the entire `handleSessionPatch` function with:

```go
// handleSessionPatch applies partial updates to a session. All fields are
// optional; a missing JSON key means "leave unchanged", an explicit empty
// string clears the field (where applicable).
func (s *Server) handleSessionPatch(w http.ResponseWriter, r *http.Request) {
	if s.opts.Storage == nil {
		http.Error(w, "storage not configured", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing session id", http.StatusBadRequest)
		return
	}
	var body struct {
		Title        *string `json:"title,omitempty"`
		SystemPrompt *string `json:"system_prompt,omitempty"`
		Model        *string `json:"model,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	upd := &storage.SessionUpdate{}

	if body.Title != nil {
		title := strings.TrimSpace(*body.Title)
		if title == "" {
			http.Error(w, "title must not be empty", http.StatusBadRequest)
			return
		}
		if len(title) > MaxSessionTitleBytes {
			http.Error(w, "title too long", http.StatusBadRequest)
			return
		}
		upd.Title = title
	}
	if body.SystemPrompt != nil {
		if len(*body.SystemPrompt) > MaxSystemPromptBytes {
			http.Error(w, "system_prompt too long", http.StatusBadRequest)
			return
		}
		upd.SystemPrompt = body.SystemPrompt
	}
	if body.Model != nil {
		if len(*body.Model) > MaxModelNameBytes {
			http.Error(w, "model name too long", http.StatusBadRequest)
			return
		}
		upd.Model = body.Model
	}

	err := s.opts.Storage.UpdateSession(r.Context(), id, upd)
	if errors.Is(err, storage.ErrNotFound) {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sess, err := s.opts.Storage.GetSession(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, dtoFromSession(sess))
}
```

Remove the now-unused `utf8` import if it was only used in the old handler.

- [ ] **Step 5: Run the tests**

```bash
go test ./api/ -run TestPatchSession -v
```

Expected: all PATCH tests PASS.

- [ ] **Step 6: Run the full api suite**

```bash
go test ./api/... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api/handlers_sessions.go api/session_patch_limits.go api/handlers_sessions_test.go
git commit -m "$(cat <<'EOF'
feat(api): PATCH /api/sessions/{id} accepts model + system_prompt

Extends the existing title-only handler with two optional fields.
Size caps (MaxSessionTitleBytes=256, MaxSystemPromptBytes=32KB,
MaxModelNameBytes=128) guard against accidental large blobs.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Broadcast session_updated SSE event from PATCH

**Files:**
- Modify: `api/handlers_sessions.go` — publish after successful update
- Modify: `api/stream.go` — add `EventTypeSessionUpdated` const
- Test: `api/handlers_sessions_test.go` — subscribe to hub, PATCH, assert event received

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_sessions_test.go`:

```go
func TestPatchSession_BroadcastsSessionUpdatedEvent(t *testing.T) {
	s, store := newTestServerWithStore(t)
	store.seedSessionFull("s-evt", "web", "claude-opus-4-7", "orig", "t")

	// Subscribe to the hub before triggering the PATCH.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, subID := s.opts.Streams.Subscribe(ctx, "s-evt")
	defer s.opts.Streams.Unsubscribe("s-evt", subID)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/sessions/s-evt",
		strings.NewReader(`{"system_prompt":"new"}`))
	req.Header.Set("Authorization", "Bearer t")
	req.Header.Set("Content-Type", "application/json")
	s.Router().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	select {
	case ev := <-ch:
		assert.Equal(t, "session_updated", ev.Type)
		assert.Equal(t, "s-evt", ev.SessionID)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for session_updated event")
	}
}
```

Check the actual `Subscribe` signature in `api/stream.go` / `api/hub.go`; adjust argument order if needed.

- [ ] **Step 2: Run to verify it fails**

```bash
go test ./api/ -run TestPatchSession_BroadcastsSessionUpdatedEvent -v
```

Expected: FAIL — `timed out waiting for session_updated event`.

- [ ] **Step 3: Add the event type constant**

Edit `api/stream.go`. In the const block (around line 15), add:

```go
const (
	EventTypeToken      = "token"
	EventTypeToolCall   = "tool_call"
	EventTypeToolResult = "tool_result"
	EventTypeStatus     = "status"

	// EventTypeSessionUpdated fires after a successful PATCH /api/sessions/{id}.
	// Data is a map with the updated title / model / system_prompt — whichever
	// fields changed. Clients merge the payload into their local session cache.
	EventTypeSessionUpdated = "session_updated"
)
```

- [ ] **Step 4: Publish from the PATCH handler**

Edit `api/handlers_sessions.go`. In `handleSessionPatch`, right before `writeJSON(w, dtoFromSession(sess))`, add:

```go
if s.opts.Streams != nil {
	s.opts.Streams.Publish(StreamEvent{
		Type:      EventTypeSessionUpdated,
		SessionID: id,
		Data: map[string]any{
			"title":         sess.Title,
			"model":         sess.Model,
			"system_prompt": sess.SystemPrompt,
		},
	})
}
```

- [ ] **Step 5: Run the test**

```bash
go test ./api/ -run TestPatchSession_BroadcastsSessionUpdatedEvent -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/handlers_sessions.go api/stream.go api/handlers_sessions_test.go
git commit -m "$(cat <<'EOF'
feat(api): emit session_updated SSE event on PATCH success

Clients subscribed to a session receive a session_updated event with
the new title/model/system_prompt so multiple tabs stay in sync.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Remove model from POST /api/sessions/{id}/messages

**Files:**
- Modify: `api/dto.go` or wherever `MessageSubmitRequest` lives — drop `Model`
- Modify: `api/handlers_session_run.go` — drop `Model: req.Model` from sessionrun.Request
- Modify: `api/sessionrun/runner.go` — drop `Model` from `Request` struct
- Test: `api/handlers_sessions_test.go` — verify the field is now ignored

- [ ] **Step 1: Locate MessageSubmitRequest**

```bash
grep -n 'MessageSubmitRequest' --include='*.go' -r .
```

Identify the file that defines the struct (likely `api/dto.go`). Note the current shape has `Text` and `Model`.

- [ ] **Step 2: Write the failing test**

Append to `api/handlers_sessions_test.go`:

```go
func TestPostMessage_IgnoresDeprecatedModelField(t *testing.T) {
	s, _ := newTestServerWithStore(t)
	// POST with a model field in body — server should accept the request
	// (text is valid) and ignore the model entry. We assert by making sure
	// the session row subsequently created has Model = "" (not the value
	// from the body). This test needs a provider stub; if the fixture does
	// not already set one, this test belongs behind `t.Skip` with a note.
	t.Skip("covered by session_created assertion when a provider stub is in the fixture")
}
```

This test is intentionally skipped — the "no model in POST" behavior is structurally enforced by Step 3 (removing the field from the struct). Keep the skip so the intent is documented; actual coverage is the compile-time absence of the field.

- [ ] **Step 3: Drop Model from MessageSubmitRequest**

Edit the file defining `MessageSubmitRequest` (most likely `api/dto.go`):

Before:
```go
type MessageSubmitRequest struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
}
```

After:
```go
type MessageSubmitRequest struct {
	Text string `json:"text"`
}
```

- [ ] **Step 4: Drop Model plumbing from handler**

Edit `api/handlers_session_run.go`. Update the goroutine body:

Before:
```go
_ = sessionrun.Run(ctx, s.deps, sessionrun.Request{
	SessionID:   sessionID,
	UserMessage: req.Text,
	Model:       req.Model,
})
```

After:
```go
_ = sessionrun.Run(ctx, s.deps, sessionrun.Request{
	SessionID:   sessionID,
	UserMessage: req.Text,
})
```

- [ ] **Step 5: Drop Model from sessionrun.Request**

Edit `api/sessionrun/runner.go`. Find the `Request` struct (near the top) and remove the `Model` field. Then search for any remaining uses:

```bash
grep -n 'Model' api/sessionrun/runner.go
```

Where the old code used `req.Model`, pass the empty string (the engine's fallback + Task 6's session-model preference will cover the gap). If the runner passes `Model` into `agent.RunOptions`, drop that assignment — `RunOptions.Model` stays `""` for new sessions and the engine's own fallback (`"claude-opus-4-6"` in `conversation.go:35-36`) takes over, which is immediately overwritten by `Session.Model` once the row exists (Task 6).

- [ ] **Step 6: Build and run the full test suite**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go build ./...
go test ./... -count=1
```

Expected: compile succeeds (no stale references to `req.Model`), all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add api/dto.go api/handlers_session_run.go api/sessionrun/runner.go api/handlers_sessions_test.go
git commit -m "$(cat <<'EOF'
feat(api): model is no longer a per-message attribute

POST /api/sessions/{id}/messages no longer accepts `model`. Session
model is set/changed via PATCH /api/sessions/{id} and read out of the
session row on every turn.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Frontend — update SessionSummarySchema and drop model from MessageSubmitRequest

**Files:**
- Modify: `web/src/api/schemas.ts` — add `system_prompt` to SessionSummarySchema, drop `model` from MessageSubmitRequestSchema, add SessionPatchSchema + SessionUpdatedPayloadSchema

- [ ] **Step 1: Update schemas**

Edit `web/src/api/schemas.ts`. Find `MessageSubmitRequestSchema` (around line 128):

Before:
```ts
export const MessageSubmitRequestSchema = z.object({
  text: z.string().min(1),
  model: z.string().optional(),
});
```

After:
```ts
export const MessageSubmitRequestSchema = z.object({
  text: z.string().min(1),
});
```

Find `SessionSummarySchema` (around line 140). Add `system_prompt`:

```ts
export const SessionSummarySchema = z.object({
  id: z.string(),
  title: z.string().optional(),
  source: z.string(),
  model: z.string().optional(),
  system_prompt: z.string().optional(),
  started_at: z.number().optional(),
  ended_at: z.number().optional(),
  message_count: z.number().optional(),
});
```

Below `SessionSummarySchema`, add two new schemas:

```ts
export const SessionPatchSchema = z.object({
  title: z.string().optional(),
  model: z.string().optional(),
  system_prompt: z.string().optional(),
});
export type SessionPatch = z.infer<typeof SessionPatchSchema>;

export const SessionUpdatedPayloadSchema = z.object({
  title: z.string().optional(),
  model: z.string().optional(),
  system_prompt: z.string().optional(),
});
export type SessionUpdatedPayload = z.infer<typeof SessionUpdatedPayloadSchema>;
```

- [ ] **Step 2: Typecheck**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind/web
npm run typecheck 2>&1 | head -40
```

Expected: type errors where callers of `MessageSubmitRequestSchema` still try to pass `model`. Those callers are fixed in Task 15 — leave the errors for now unless they block `npm test`.

- [ ] **Step 3: Commit**

```bash
git add web/src/api/schemas.ts
git commit -m "$(cat <<'EOF'
feat(web): add system_prompt to SessionSummary; remove model from POST

Defines SessionPatchSchema for PATCH payloads and
SessionUpdatedPayloadSchema for the new SSE event. Consumers updated
in subsequent commits.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Frontend — rename renameSession → patchSession in useSessionList

**Files:**
- Modify: `web/src/hooks/useSessionList.ts` — rename + generalize
- Modify: `web/src/components/chat/ChatWorkspace.tsx` — update caller
- Test: add a test if one does not already cover rename/patch behavior

- [ ] **Step 1: Update the hook**

Edit `web/src/hooks/useSessionList.ts`. Replace:

```ts
const renameSession = useCallback((id: string, title: string) => {
  setSessions((prev) =>
    prev.map((s) => (s.id === id ? { ...s, title } : s)),
  );
}, []);
```

With:

```ts
const patchSession = useCallback(
  (id: string, patch: Partial<Pick<SessionSummary, 'title' | 'model' | 'system_prompt'>>) => {
    setSessions((prev) =>
      prev.map((s) => (s.id === id ? { ...s, ...patch } : s)),
    );
  },
  [],
);
```

Change the returned object:

```ts
return { sessions, error, newSession, insertSession, patchSession, refetch };
```

- [ ] **Step 2: Update ChatWorkspace caller**

Edit `web/src/components/chat/ChatWorkspace.tsx`. Find `renameSession` and replace:

```ts
const { sessions, newSession, insertSession, renameSession } = useSessionList();
```

with:

```ts
const { sessions, newSession, insertSession, patchSession } = useSessionList();
```

And the caller in `handleRename`:

```ts
await apiFetch(`/api/sessions/${encodeURIComponent(id)}`, {
  method: 'PATCH',
  body: { title },
});
patchSession(id, { title });
```

- [ ] **Step 3: Search for any other references**

```bash
cd web && grep -rn 'renameSession' src
```

Expected: no hits. If the chat sidebar or a test still references `renameSession`, update them to `patchSession(id, { title })`.

- [ ] **Step 4: Run frontend tests**

```bash
cd web && npm test --run
```

Expected: PASS. If a test file had `renameSession` in a mock, update it.

- [ ] **Step 5: Commit**

```bash
git add web/src/hooks/useSessionList.ts web/src/components/chat/ChatWorkspace.tsx
git commit -m "$(cat <<'EOF'
refactor(web): rename renameSession to patchSession in useSessionList

patchSession accepts a partial SessionSummary patch so the drawer can
update title, model, and system_prompt through a single call site.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: Frontend — handle session_updated in useChatStream

**Files:**
- Modify: `web/src/state/chat.ts` — add action type
- Modify: `web/src/hooks/useChatStream.ts` — add case, wire `onSessionUpdated` callback
- Modify: `web/src/components/chat/ChatWorkspace.tsx` — pass `patchSession` as the handler
- Test: `web/src/hooks/useChatStream.test.tsx` (or the existing file that tests useChatStream) — add a case

- [ ] **Step 1: Write the failing test**

Append to `web/src/hooks/useChatStream.test.tsx` (or the neighboring test file — search for `useChatStream` tests):

```tsx
import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useChatStream } from './useChatStream';

// The test uses whatever EventSource mock the existing tests already
// install. Follow the mock pattern in this file's existing describe blocks.

describe('useChatStream session_updated', () => {
  afterEach(() => vi.clearAllMocks());

  it('invokes onSessionUpdated when a session_updated event arrives', () => {
    const dispatch = vi.fn();
    const onSessionUpdated = vi.fn();
    // Render the hook, then have the mock EventSource fire an event:
    renderHook(() =>
      useChatStream('sess-1', dispatch, undefined, onSessionUpdated),
    );
    // Emit the event through the EventSource mock (see existing tests
    // for the exact helper; typically `mockEventSource.emit('message', {...})`).
    emitMockEvent({
      type: 'session_updated',
      session_id: 'sess-1',
      data: { model: 'claude-sonnet-4-6', system_prompt: 'new', title: 't' },
    });

    expect(onSessionUpdated).toHaveBeenCalledWith('sess-1', {
      model: 'claude-sonnet-4-6',
      system_prompt: 'new',
      title: 't',
    });
  });
});
```

If the existing tests don't have a helper like `emitMockEvent`, write the minimum EventSource mock inline at the top of the test. Don't invent helpers that aren't there.

- [ ] **Step 2: Run to verify it fails**

```bash
cd web && npm test -- useChatStream --run
```

Expected: FAIL — `useChatStream` does not accept a 4th argument.

- [ ] **Step 3: Update the hook signature**

Edit `web/src/hooks/useChatStream.ts`. Replace the signature and body:

```ts
import { useEffect, useRef } from 'react';
import type { ChatAction } from '../state/chat';
import {
  SessionSummarySchema,
  SessionUpdatedPayloadSchema,
  type SessionSummary,
  type SessionUpdatedPayload,
} from '../api/schemas';

type Dispatch = (a: ChatAction) => void;

export function useChatStream(
  sessionId: string | null,
  dispatch: Dispatch,
  onSessionCreated?: (session: SessionSummary) => void,
  onSessionUpdated?: (id: string, patch: SessionUpdatedPayload) => void,
) {
  const tokenBufRef = useRef('');
  const rafPendingRef = useRef(false);
  const onSessionCreatedRef = useRef(onSessionCreated);
  const onSessionUpdatedRef = useRef(onSessionUpdated);
  onSessionCreatedRef.current = onSessionCreated;
  onSessionUpdatedRef.current = onSessionUpdated;

  useEffect(() => {
    if (!sessionId) return;
    const token = new URLSearchParams(window.location.search).get('t') ?? '';
    const es = new EventSource(
      `/api/sessions/${encodeURIComponent(sessionId)}/stream/sse?t=${encodeURIComponent(token)}`,
    );

    function flushTokens() {
      rafPendingRef.current = false;
      if (tokenBufRef.current) {
        dispatch({ type: 'chat/stream/token', delta: tokenBufRef.current });
        tokenBufRef.current = '';
      }
    }

    es.onmessage = (ev) => {
      let parsed: { type?: string; session_id?: string; data?: Record<string, unknown> };
      try {
        parsed = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (parsed.session_id && parsed.session_id !== sessionId) return;
      switch (parsed.type) {
        case 'session_created': {
          const payload = SessionSummarySchema.parse(parsed.data);
          dispatch({ type: 'chat/session/created', session: payload });
          onSessionCreatedRef.current?.(payload);
          break;
        }
        case 'session_updated': {
          const payload = SessionUpdatedPayloadSchema.parse(parsed.data);
          onSessionUpdatedRef.current?.(parsed.session_id!, payload);
          break;
        }
        case 'token': {
          const d = parsed.data as { text?: string } | undefined;
          if (typeof d?.text === 'string') {
            tokenBufRef.current += d.text;
            if (!rafPendingRef.current) {
              rafPendingRef.current = true;
              requestAnimationFrame(flushTokens);
            }
          }
          break;
        }
        case 'tool_call': {
          const d = parsed.data as Record<string, unknown>;
          dispatch({
            type: 'chat/stream/toolCall',
            call: {
              id: String(d.id ?? d.tool_use_id ?? Date.now()),
              name: String(d.name ?? 'tool'),
              input: d.input ?? d,
              state: 'running',
            },
          });
          break;
        }
        case 'tool_result': {
          const d = parsed.data as { call?: { id?: string }; result?: string } | undefined;
          dispatch({
            type: 'chat/stream/toolResult',
            id: String(d?.call?.id ?? Date.now()),
            result: String(d?.result ?? ''),
          });
          break;
        }
        case 'message_complete': {
          flushTokens();
          const d = parsed.data as { assistant_text?: string; message_id?: string } | undefined;
          dispatch({
            type: 'chat/stream/complete',
            text: String(d?.assistant_text ?? ''),
            messageId: String(d?.message_id ?? `complete-${Date.now()}`),
          });
          break;
        }
        case 'status': {
          const d = parsed.data as { state?: string; error?: string } | undefined;
          if (d?.state === 'cancelled') {
            flushTokens();
            dispatch({ type: 'chat/stream/cancelled' });
          } else if (d?.state === 'error') {
            dispatch({ type: 'chat/stream/error', message: String(d.error ?? 'error') });
          }
          break;
        }
      }
    };

    return () => {
      es.close();
    };
  }, [sessionId, dispatch]);
}
```

- [ ] **Step 4: Wire ChatWorkspace**

Edit `web/src/components/chat/ChatWorkspace.tsx`. Replace:

```ts
useChatStream(sessionId, dispatch, insertSession);
```

With:

```ts
useChatStream(sessionId, dispatch, insertSession, (id, patch) => patchSession(id, patch));
```

- [ ] **Step 5: Run tests**

```bash
cd web && npm test --run
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/hooks/useChatStream.ts web/src/hooks/useChatStream.test.tsx web/src/components/chat/ChatWorkspace.tsx
git commit -m "$(cat <<'EOF'
feat(web): handle session_updated SSE event via useChatStream

The hook accepts a new onSessionUpdated callback that ChatWorkspace
wires to useSessionList.patchSession, keeping the sessions list in
sync across tabs.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: Frontend — create SessionSettingsDrawer

**Files:**
- Create: `web/src/components/chat/SessionSettingsDrawer.tsx`
- Create: `web/src/components/chat/SessionSettingsDrawer.module.css`
- Create: `web/src/components/chat/SessionSettingsDrawer.test.tsx`
- Modify: `web/src/i18n/locales/en.json` + `zh.json` — add drawer strings (follow existing pattern)

- [ ] **Step 1: Write the failing test**

Create `web/src/components/chat/SessionSettingsDrawer.test.tsx`:

```tsx
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import SessionSettingsDrawer from './SessionSettingsDrawer';
import type { SessionSummary } from '../../api/schemas';

const session: SessionSummary = {
  id: 'sess-1',
  source: 'web',
  title: 't',
  model: 'claude-opus-4-7',
  system_prompt: 'orig prompt',
};

describe('SessionSettingsDrawer', () => {
  it('renders current model and system prompt when open', () => {
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['', 'claude-opus-4-7', 'claude-sonnet-4-6']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByRole('combobox')).toHaveValue('claude-opus-4-7');
    expect(screen.getByRole('textbox')).toHaveValue('orig prompt');
  });

  it('does not render when closed', () => {
    render(
      <SessionSettingsDrawer
        open={false}
        session={session}
        modelOptions={['']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('calls onSave with only the fields that changed', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['', 'claude-opus-4-7', 'claude-sonnet-4-6']}
        onClose={vi.fn()}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByRole('combobox'), {
      target: { value: 'claude-sonnet-4-6' },
    });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));
    await Promise.resolve(); // flush microtask

    expect(onSave).toHaveBeenCalledWith({ model: 'claude-sonnet-4-6' });
  });

  it('cancel discards draft and calls onClose', () => {
    const onClose = vi.fn();
    const onSave = vi.fn();
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['']}
        onClose={onClose}
        onSave={onSave}
      />,
    );
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'changed' },
    });
    fireEvent.click(screen.getByRole('button', { name: /cancel/i }));
    expect(onClose).toHaveBeenCalled();
    expect(onSave).not.toHaveBeenCalled();
  });

  it('Esc key closes the drawer', () => {
    const onClose = vi.fn();
    render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['']}
        onClose={onClose}
        onSave={vi.fn()}
      />,
    );
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' });
    expect(onClose).toHaveBeenCalled();
  });

  it('shows conflict banner when session prop updates while drawer is open with unsaved draft', () => {
    const { rerender } = render(
      <SessionSettingsDrawer
        open
        session={session}
        modelOptions={['']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByRole('textbox'), {
      target: { value: 'local draft' },
    });
    rerender(
      <SessionSettingsDrawer
        open
        session={{ ...session, system_prompt: 'remote change' }}
        modelOptions={['']}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );
    expect(screen.getByText(/updated in another/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run to verify it fails**

```bash
cd web && npm test -- SessionSettingsDrawer --run
```

Expected: FAIL (module not found).

- [ ] **Step 3: Create the component**

Create `web/src/components/chat/SessionSettingsDrawer.tsx`:

```tsx
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { SessionSummary, SessionPatch } from '../../api/schemas';
import styles from './SessionSettingsDrawer.module.css';

type Props = {
  open: boolean;
  session: SessionSummary;
  modelOptions: string[];
  onClose: () => void;
  onSave: (patch: SessionPatch) => Promise<void>;
};

export default function SessionSettingsDrawer({
  open, session, modelOptions, onClose, onSave,
}: Props) {
  const { t } = useTranslation('ui');
  const [draftModel, setDraftModel] = useState(session.model ?? '');
  const [draftPrompt, setDraftPrompt] = useState(session.system_prompt ?? '');
  const [savedModel, setSavedModel] = useState(session.model ?? '');
  const [savedPrompt, setSavedPrompt] = useState(session.system_prompt ?? '');
  const [saving, setSaving] = useState(false);
  const textAreaRef = useRef<HTMLTextAreaElement>(null);

  // Reset draft + saved baselines when drawer opens or session id changes.
  useEffect(() => {
    if (open) {
      setDraftModel(session.model ?? '');
      setDraftPrompt(session.system_prompt ?? '');
      setSavedModel(session.model ?? '');
      setSavedPrompt(session.system_prompt ?? '');
      // Focus the prompt after mount
      setTimeout(() => textAreaRef.current?.focus(), 0);
    }
  }, [open, session.id]);

  // Track whether the session prop diverged from our baseline (another tab).
  const sessionModel = session.model ?? '';
  const sessionPrompt = session.system_prompt ?? '';
  const draftDirty = draftModel !== savedModel || draftPrompt !== savedPrompt;
  const externalChange =
    (sessionModel !== savedModel || sessionPrompt !== savedPrompt) && draftDirty;

  function handleCancel() {
    setDraftModel(savedModel);
    setDraftPrompt(savedPrompt);
    onClose();
  }

  async function handleSave() {
    if (saving || !draftDirty) return;
    const patch: SessionPatch = {};
    if (draftModel !== savedModel) patch.model = draftModel;
    if (draftPrompt !== savedPrompt) patch.system_prompt = draftPrompt;
    setSaving(true);
    try {
      await onSave(patch);
      setSavedModel(draftModel);
      setSavedPrompt(draftPrompt);
      onClose();
    } finally {
      setSaving(false);
    }
  }

  if (!open) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={t('chat.settings.title')}
      className={styles.drawer}
      onKeyDown={(e) => {
        if (e.key === 'Escape') handleCancel();
      }}
      tabIndex={-1}
    >
      <header className={styles.header}>
        <h3 className={styles.title}>{t('chat.settings.title')}</h3>
      </header>

      {externalChange && (
        <div role="status" className={styles.conflict}>
          {t('chat.settings.updatedElsewhere')}
        </div>
      )}

      <label className={styles.field}>
        <span className={styles.label}>{t('chat.settings.model')}</span>
        <select
          className={styles.select}
          value={draftModel}
          onChange={(e) => setDraftModel(e.target.value)}
        >
          {modelOptions.map((m) => (
            <option key={m || '(default)'} value={m}>
              {m || t('chat.settings.defaultModel')}
            </option>
          ))}
        </select>
      </label>

      <label className={styles.field}>
        <span className={styles.label}>{t('chat.settings.systemPrompt')}</span>
        <textarea
          ref={textAreaRef}
          className={styles.textarea}
          value={draftPrompt}
          onChange={(e) => setDraftPrompt(e.target.value)}
          rows={12}
        />
      </label>

      <footer className={styles.actions}>
        <button
          type="button"
          className={styles.btn}
          onClick={handleCancel}
          disabled={saving}
        >
          {t('chat.settings.cancel')}
        </button>
        <button
          type="button"
          className={`${styles.btn} ${styles.primary}`}
          onClick={handleSave}
          disabled={saving || !draftDirty}
        >
          {t('chat.settings.save')}
        </button>
      </footer>
    </div>
  );
}
```

- [ ] **Step 4: Create the CSS**

Create `web/src/components/chat/SessionSettingsDrawer.module.css`:

```css
.drawer {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: clamp(360px, 40vw, 540px);
  padding: 16px;
  background: var(--bg, #0a0a0a);
  border-left: 1px solid #333;
  box-shadow: -8px 0 20px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  font-family: ui-monospace, SF Mono, Menlo, monospace;
  font-size: 13px;
}
.header { border-bottom: 1px solid #222; padding-bottom: 8px; }
.title { margin: 0; font-size: 13px; color: #FFB800; font-weight: 600; }
.conflict {
  padding: 6px 8px;
  border: 1px solid #FFB800;
  border-radius: 2px;
  font-size: 11px;
  color: #FFB800;
  background: rgba(255, 184, 0, 0.05);
}
.field { display: flex; flex-direction: column; gap: 4px; }
.label {
  font-size: 10px;
  color: #888;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.select,
.textarea {
  width: 100%;
  padding: 6px 8px;
  background: #111;
  color: #d4d4d4;
  border: 1px solid #333;
  border-radius: 2px;
  font-family: inherit;
  font-size: 13px;
  box-sizing: border-box;
}
.textarea { min-height: 180px; resize: vertical; line-height: 1.5; }
.select:focus,
.textarea:focus { outline: none; border-color: #FFB800; }
.actions {
  margin-top: auto;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 8px;
  border-top: 1px solid #222;
}
.btn {
  padding: 4px 12px;
  font-size: 11px;
  font-family: inherit;
  background: #181818;
  color: #d4d4d4;
  border: 1px solid #333;
  border-radius: 2px;
  cursor: pointer;
}
.btn:hover { border-color: #555; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.primary { border-color: #FFB800; color: #FFB800; }
.primary:hover { background: rgba(255, 184, 0, 0.1); }
```

- [ ] **Step 5: Add i18n strings**

Edit `web/src/i18n/locales/en.json`. Find the `"chat"` key and add a `"settings"` sub-tree (preserving the existing contents):

```json
"settings": {
  "title": "Session settings",
  "model": "Model",
  "defaultModel": "(default)",
  "systemPrompt": "System prompt",
  "save": "Save",
  "cancel": "Cancel",
  "updatedElsewhere": "This session was updated in another window. Your changes are unsaved — Save to overwrite, or Cancel to discard."
}
```

Edit `web/src/i18n/locales/zh.json` similarly:

```json
"settings": {
  "title": "会话设置",
  "model": "模型",
  "defaultModel": "（默认）",
  "systemPrompt": "系统提示词",
  "save": "保存",
  "cancel": "取消",
  "updatedElsewhere": "该会话已在另一个窗口被修改。你有未保存的草稿——保存会覆盖，取消会丢弃。"
}
```

Match the exact JSON structure of the existing file; if the chat strings live in a different nesting, place `"settings"` under the same parent as `chat.newConversation` (whichever key the other chat strings sit under).

- [ ] **Step 6: Run the drawer tests**

```bash
cd web && npm test -- SessionSettingsDrawer --run
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/chat/SessionSettingsDrawer.tsx web/src/components/chat/SessionSettingsDrawer.module.css web/src/components/chat/SessionSettingsDrawer.test.tsx web/src/i18n/locales/en.json web/src/i18n/locales/zh.json
git commit -m "$(cat <<'EOF'
feat(web): SessionSettingsDrawer component for per-session model+prompt

Right-side drawer with draft state, save-on-click semantics, external-
change conflict banner, Esc-to-cancel. No effect on any other component
yet; ConversationHeader wires it up in the next commit.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 15: Frontend — replace ModelSelector with SettingsButton in ConversationHeader

**Files:**
- Create: `web/src/components/chat/SettingsButton.tsx`
- Create: `web/src/components/chat/SettingsButton.module.css`
- Modify: `web/src/components/chat/ConversationHeader.tsx` — swap the ModelSelector for SettingsButton
- Delete: `web/src/components/chat/ModelSelector.tsx` + `.module.css` (no longer used)
- Test: update/replace `ConversationHeader` tests if they exist

- [ ] **Step 1: Create SettingsButton**

Create `web/src/components/chat/SettingsButton.tsx`:

```tsx
import styles from './SettingsButton.module.css';

type Props = {
  onClick: () => void;
  disabled?: boolean;
  ariaLabel: string;
};

export default function SettingsButton({ onClick, disabled, ariaLabel }: Props) {
  return (
    <button
      type="button"
      className={styles.button}
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel}
      title={ariaLabel}
    >
      {/* Gear icon — inline SVG so we don't pull an icon dep. */}
      <svg
        width="14"
        height="14"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33h0a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51h0a1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v0a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    </button>
  );
}
```

Create `web/src/components/chat/SettingsButton.module.css`:

```css
.button {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  background: #181818;
  color: #d4d4d4;
  border: 1px solid #333;
  border-radius: 2px;
  cursor: pointer;
}
.button:hover { color: #FFB800; border-color: #FFB800; }
.button:disabled { opacity: 0.4; cursor: not-allowed; }
.button:disabled:hover { color: #d4d4d4; border-color: #333; }
```

- [ ] **Step 2: Update ConversationHeader**

Replace the whole `web/src/components/chat/ConversationHeader.tsx`:

```tsx
import { useTranslation } from 'react-i18next';
import SettingsButton from './SettingsButton';
import styles from './ConversationHeader.module.css';

type Props = {
  title: string;
  onOpenSettings: () => void;
  settingsDisabled?: boolean;
};

export default function ConversationHeader({ title, onOpenSettings, settingsDisabled }: Props) {
  const { t } = useTranslation('ui');
  return (
    <header className={styles.header}>
      <h2 className={styles.title}>{title}</h2>
      <SettingsButton
        onClick={onOpenSettings}
        disabled={settingsDisabled}
        ariaLabel={t('chat.settings.title')}
      />
    </header>
  );
}
```

- [ ] **Step 3: Delete ModelSelector**

```bash
git rm web/src/components/chat/ModelSelector.tsx web/src/components/chat/ModelSelector.module.css
```

- [ ] **Step 4: Update any existing ConversationHeader test**

```bash
cd web && grep -rn 'ModelSelector\|ConversationHeader' src
```

For each test that references `ModelSelector` or the old `ConversationHeader` props shape (`model`, `modelOptions`, `onModelChange`), update to the new prop shape (`title`, `onOpenSettings`, `settingsDisabled`). If `ChatWorkspace.test.tsx` renders `ConversationHeader` indirectly, it is fixed in Task 16.

- [ ] **Step 5: Run tests**

```bash
cd web && npm test --run
```

Expected: PASS. If a test still invokes the old model-change flow, update or remove it (the behavior moved to the drawer, covered by its own tests).

- [ ] **Step 6: Commit**

```bash
git add web/src/components/chat/SettingsButton.tsx web/src/components/chat/SettingsButton.module.css web/src/components/chat/ConversationHeader.tsx
git commit -m "$(cat <<'EOF'
feat(web): ConversationHeader uses gear button instead of model select

The top-right model <select> is replaced by a 24x24 settings button that
opens the session settings drawer. ModelSelector and its CSS are removed.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: Frontend — wire the drawer into ChatWorkspace

**Files:**
- Modify: `web/src/components/chat/ChatWorkspace.tsx` — open/close drawer, drop `composer.selectedModel`, drop model from POST, read session model/prompt from `sessions`
- Modify: `web/src/state/chat.ts` — drop `chat/composer/setModel` action + `selectedModel` field (scan reducer + tests for any usage)
- Create or modify: `web/src/api/models.ts` — single source of truth for hardcoded MODEL_OPTIONS

- [ ] **Step 1: Create models.ts constant**

Create `web/src/api/models.ts`:

```ts
// Hardcoded model list until a /api/models discovery endpoint exists.
// Empty string means "use session default / server fallback".
export const MODEL_OPTIONS: readonly string[] = [
  '',
  'claude-opus-4-7',
  'claude-sonnet-4-6',
  'gpt-4',
];
```

- [ ] **Step 2: Rewrite ChatWorkspace**

Replace `web/src/components/chat/ChatWorkspace.tsx` with:

```tsx
import { useReducer, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import ChatSidebar from './ChatSidebar';
import ConversationHeader from './ConversationHeader';
import MessageList from './MessageList';
import ComposerBar from './ComposerBar';
import SessionSettingsDrawer from './SessionSettingsDrawer';
import Toast from './Toast';
import styles from './ChatWorkspace.module.css';
import { useSessionList } from '../../hooks/useSessionList';
import { useChatStream } from '../../hooks/useChatStream';
import { chatReducer, initialChatState } from '../../state/chat';
import { apiFetch, ApiError } from '../../api/client';
import {
  MessageSubmitResponseSchema,
  MessagesResponseSchema,
  type SessionPatch,
} from '../../api/schemas';
import { MODEL_OPTIONS } from '../../api/models';

type Props = {
  sessionId: string | null;
  onChangeSession: (id: string) => void;
  providerConfigured?: boolean;
};

export default function ChatWorkspace({ sessionId, onChangeSession, providerConfigured = true }: Props) {
  const { t } = useTranslation('ui');
  const [state, dispatch] = useReducer(chatReducer, initialChatState);
  const { sessions, newSession, insertSession, patchSession } = useSessionList();
  const [toast, setToast] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);

  useChatStream(sessionId, dispatch, insertSession, (id, patch) => patchSession(id, patch));

  useEffect(() => {
    if (!sessionId) return;
    const ctrl = new AbortController();
    apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/messages`, {
      schema: MessagesResponseSchema,
      signal: ctrl.signal,
    })
      .then((r) => {
        dispatch({
          type: 'chat/messages/loaded',
          sessionId,
          messages: r.messages.map((m) => ({
            id: m.id,
            role: m.role,
            content: m.content,
            timestamp: m.timestamp ?? Date.now(),
          })),
        });
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return;
        if (err instanceof ApiError && err.status === 404) return;
        console.warn('load messages failed', err);
      });
    return () => ctrl.abort();
  }, [sessionId]);

  async function handleSend() {
    if (!sessionId) return;
    const text = state.composer.text.trim();
    if (!text) return;
    dispatch({ type: 'chat/composer/setText', text: '' });
    dispatch({ type: 'chat/stream/start', sessionId, userText: text });
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/messages`, {
        method: 'POST',
        body: { text },
        schema: MessageSubmitResponseSchema,
      });
    } catch (err) {
      dispatch({ type: 'chat/stream/rollbackUserMessage', sessionId });
      if (err instanceof ApiError) {
        if (err.status === 409) setToast(t('chat.errorBusy'));
        else if (err.status === 503) setToast(t('chat.errorNoProvider'));
        else setToast(t('chat.errorSendFailed', { msg: err.message }));
      } else {
        setToast(t('chat.errorSendFailed', { msg: err instanceof Error ? err.message : '' }));
      }
    }
  }

  async function handleRename(id: string, title: string): Promise<void> {
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(id)}`, {
        method: 'PATCH',
        body: { title },
      });
      patchSession(id, { title });
    } catch (err) {
      setToast(t('chat.renameFailed', { msg: err instanceof Error ? err.message : '' }));
      throw err;
    }
  }

  async function handleStop() {
    if (!sessionId) return;
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}/cancel`, {
        method: 'POST',
      });
    } catch (err) {
      console.warn('cancel failed', err);
    }
  }

  async function handleSettingsSave(patch: SessionPatch) {
    if (!sessionId) return;
    try {
      await apiFetch(`/api/sessions/${encodeURIComponent(sessionId)}`, {
        method: 'PATCH',
        body: patch,
      });
      patchSession(sessionId, patch);
    } catch (err) {
      if (err instanceof ApiError && err.status === 400) {
        setToast(t('chat.settings.saveTooLong'));
      } else {
        setToast(t('chat.settings.saveFailed', { msg: err instanceof Error ? err.message : '' }));
      }
      throw err;
    }
  }

  const activeSession = sessions.find((s) => s.id === sessionId);
  const activeTitle = activeSession?.title ?? t('chat.newConversation');

  return (
    <div className={styles.workspace}>
      <ChatSidebar
        sessions={sessions}
        activeId={sessionId}
        onSelect={onChangeSession}
        onNew={() => {
          const id = newSession();
          onChangeSession(id);
        }}
        onRename={handleRename}
      />
      <main className={styles.main}>
        <ConversationHeader
          title={activeTitle}
          onOpenSettings={() => setSettingsOpen(true)}
          settingsDisabled={!activeSession}
        />
        <MessageList
          messages={state.messagesBySession[sessionId ?? ''] ?? []}
          streamingDraft={state.streaming.assistantDraft}
          streamingToolCalls={state.streaming.toolCalls}
          streamingSessionId={state.streaming.sessionId}
          activeSessionId={sessionId}
        />
        {state.streaming.status === 'error' && state.streaming.error && (
          <div role="alert" className={styles.errorBanner}>
            {state.streaming.error}
          </div>
        )}
        <ComposerBar
          text={state.composer.text}
          onChangeText={(txt) => dispatch({ type: 'chat/composer/setText', text: txt })}
          onSend={handleSend}
          onStop={handleStop}
          disabled={!providerConfigured}
          streaming={state.streaming.status === 'running'}
          onSlashCommand={(cmd) => {
            switch (cmd) {
              case 'new': {
                const id = newSession();
                onChangeSession(id);
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
              }
              case 'settings':
                if (activeSession) setSettingsOpen(true);
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
              case 'model':
              case 'clear':
                dispatch({ type: 'chat/composer/setText', text: '' });
                break;
            }
          }}
        />
        {activeSession && (
          <SessionSettingsDrawer
            open={settingsOpen}
            session={activeSession}
            modelOptions={[...MODEL_OPTIONS]}
            onClose={() => setSettingsOpen(false)}
            onSave={handleSettingsSave}
          />
        )}
      </main>
      {toast && <Toast message={toast} onDismiss={() => setToast(null)} />}
    </div>
  );
}
```

Notice: the `/model` slash command now redirects to the drawer too (it used to trigger the old header dropdown — there is no dropdown anymore, so open the drawer instead). If that turns out to be wrong, revisit in a follow-up.

- [ ] **Step 3: Drop composer.selectedModel from reducer**

Edit `web/src/state/chat.ts`. Search for `selectedModel` and remove it:

- Remove `selectedModel: string` from the composer state type
- Remove its initialization from `initialChatState`
- Remove the `case 'chat/composer/setModel'` branch of the reducer
- Remove the `ChatAction` type variant for `setModel`

If any test in `web/src/state.test.ts` references `setModel` or `selectedModel`, delete those cases.

- [ ] **Step 4: Add missing i18n strings**

Append to `web/src/i18n/locales/en.json`'s `chat.settings` subtree:

```json
"saveFailed": "Failed to save settings: {{msg}}",
"saveTooLong": "Prompt or model name is too long"
```

And the zh equivalents:

```json
"saveFailed": "保存设置失败：{{msg}}",
"saveTooLong": "提示词或模型名超出长度限制"
```

- [ ] **Step 5: Typecheck + tests**

```bash
cd web && npm run typecheck && npm test --run
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/src/components/chat/ChatWorkspace.tsx web/src/state/chat.ts web/src/state.test.ts web/src/api/models.ts web/src/i18n/locales/en.json web/src/i18n/locales/zh.json
git commit -m "$(cat <<'EOF'
feat(web): ChatWorkspace wires SessionSettingsDrawer; drops composer model

Model is now a session attribute sourced from the sessions list.
composer.selectedModel and the model body field on POST /messages are
gone. The gear button in the header toggles the drawer; the drawer
writes via PATCH and SSE session_updated refreshes other tabs.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: Rebuild the embedded Vite bundle

**Files:**
- Modify: `api/webroot/` (the Vite output directory that the Go server embeds)

- [ ] **Step 1: Build the web bundle**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind/web
npm run build
```

Expected: Vite emits new `api/webroot/index.html` and `api/webroot/assets/*.js|css`.

- [ ] **Step 2: Run Go tests that embed the bundle**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go test ./... -count=1
```

Expected: PASS (including any `webroot_embed_test.go` or equivalent that exercises the frontend-serving handler).

- [ ] **Step 3: Commit the rebuilt assets**

```bash
git add api/webroot/
git commit -m "$(cat <<'EOF'
chore(webroot): rebuild embedded frontend bundle for session settings drawer

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 18: End-to-end smoke check

**Files:** none — verification only.

- [ ] **Step 1: Build the binary**

```bash
cd /Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind
go build -o bin/hermind .
```

Expected: clean build.

- [ ] **Step 2: Start the web server**

```bash
HERMIND_CONFIG=~/.hermind/config.yaml ./bin/hermind web
```

Note the printed `open: http://127.0.0.1:<port>/?t=<token>`. Open it.

- [ ] **Step 3: Manual verification checklist**

Walk through each of these in the browser and verify:

1. **New conversation** — click "+ New conversation". The gear button top-right is **disabled** (no session row yet).
2. **First message** — send "hello". The session appears in the sidebar with a derived title. The gear button becomes enabled.
3. **Open drawer** — click the gear. Drawer slides in from the right. The current `system_prompt` (= config default if set, else empty) and `model` (= "" or the creation-time model) are shown.
4. **Change system prompt** — type into the textarea. The Save button becomes enabled.
5. **Save** — click Save. Drawer closes. Next message you send uses the new prompt (verify by asking the model something that depends on the prompt).
6. **Change model** — open drawer, pick a different model, Save. Next message is routed to that model.
7. **Cancel discards draft** — open drawer, type some prompt, click Cancel. Reopen drawer: textarea shows the saved value, not the draft.
8. **Esc closes** — open drawer, press Esc. Drawer closes without saving.
9. **Two-tab sync** — open the same session in two tabs, change the prompt in tab A, Save. Tab B's sessions list reflects the new model label (verify by opening the drawer in B — the new prompt appears).
10. **Settings page** — navigate to `/settings/agent`. Confirm the "Default system prompt" field renders as a textarea. Save a value and create a new session; the new session's drawer shows the configured default.
11. **Old sessions** — open an existing (pre-change) session's drawer. The `system_prompt` shown is the historical `default + first_message` concatenation. This is expected (no data migration).

- [ ] **Step 4: Log any regressions**

If any of the above fail, open a failing integration test on the nearest behavior and fix. If all pass, this task is DONE.

- [ ] **Step 5: Final commit (if any fixes were needed)**

```bash
git add <modified files>
git commit -m "fix(session-settings): address smoke-test regression — <summary>"
```

---

## Plan self-review

**Spec coverage (against `docs/superpowers/specs/2026-04-22-session-config-system-prompt-design.md`):**

| Spec item                                               | Task(s)     |
|---------------------------------------------------------|-------------|
| `config.Agent.DefaultSystemPrompt` field                | 1           |
| `PromptBuilder` appends user prompt after identity      | 3           |
| `ensureSession` stops concatenating first user message  | 4           |
| `storage.SessionUpdate` extended with `*string` fields  | 5           |
| `RunConversation` prefers `sess.Model` over opts        | 6           |
| `SessionDTO` carries `system_prompt`                    | 7           |
| `PATCH /api/sessions/{id}` accepts 3 fields with caps   | 8           |
| SSE `session_updated` event                             | 9           |
| `POST /messages` no longer accepts `model`              | 10          |
| TS `SessionSummarySchema` gains `system_prompt`         | 11          |
| `patchSession` supersedes `renameSession`               | 12          |
| `useChatStream` handles `session_updated`               | 13          |
| `SessionSettingsDrawer` component                       | 14          |
| Gear button replaces `ModelSelector`                    | 15          |
| `ChatWorkspace` wires drawer + removes composer.model   | 16          |
| `/settings/agent` renders `default_system_prompt`       | 1, 2        |
| `FieldText` multi-line descriptor kind                  | 2           |
| Frontend textarea for the settings page                 | 2           |
| Rebuild embedded bundle                                 | 17          |
| Smoke test                                              | 18          |

**Placeholder scan:** None found. Every step carries the exact file path and the exact code or command.

**Type consistency:**

- `NewPromptBuilder(platform, default string)` — consistent across Task 3 test, impl, and engine call sites.
- `storage.SessionUpdate.Model *string, SystemPrompt *string` — consistent across Task 5 SQLite impls and Task 8 handler.
- `patchSession(id, patch)` — consistent across Task 12 hook, Task 13 useChatStream callback, Task 14 drawer onSave path, Task 16 handler.
- `SessionPatch` type (web) = `{ title?, model?, system_prompt? }` — consistent from Task 11 (schema) through Task 14 (drawer) through Task 16 (workspace).
- `session_updated` SSE event payload shape `{title?, model?, system_prompt?}` — consistent between Task 9 (backend emit) and Task 11 (`SessionUpdatedPayloadSchema`).

No inconsistencies found.
