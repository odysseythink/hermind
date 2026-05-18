# Tools Toggle in Skills Page — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Tools" panel to the Skills settings page with per-tool enable/disable toggles. Disabled tools are hidden from the LLM (filtered out of function definitions) while still visible in the UI so users can re-enable them.

**Architecture:** A new `tools.disabled` list in config drives runtime filtering. The server builds a filtered `tool.Registry` copy on each conversation turn by reading the live config. The frontend's `SkillsSection` fetches the tool list from `GET /api/tools` and dispatches cross-section config updates via a new `onSectionField` callback.

**Tech Stack:** Go 1.23 (chi, yaml.v3), React + TypeScript + CSS Modules + Vite, pantheon tool registry

---

## File Map

| File | Responsibility |
|------|---------------|
| `config/config.go` | Add `ToolsConfig` struct and `Config.Tools` field |
| `config/descriptor/tools.go` | **New** — descriptor for the `tools` config section (GroupID = "skills") |
| `api/dto.go` | Extend `ToolDTO` with `Toolset` and `Enabled` fields |
| `api/server.go` | Add `disabledTools()` and `activeToolReg()` helpers; update `RunTurn` to use filtered registry |
| `api/handlers_tools.go` | Implement `handleToolsList` to return real tool list with enabled status |
| `api/handlers_conversation.go` | Update `handleConversationPost` to use `s.activeToolReg()` |
| `api/handlers_tools_test.go` | **New** — unit tests for tool listing and active registry filtering |
| `web/src/api/schemas.ts` | Extend `ToolSchema` with `toolset` and `enabled` |
| `web/src/components/groups/skills/SkillsSection.tsx` | Extend props with `onSectionField`, add tool panel state + UI |
| `web/src/components/groups/skills/SkillsSection.test.tsx` | Extend tests to cover tool panel rendering and toggle callbacks |
| `web/src/components/shell/SettingsPanel.tsx` | Pass `onSectionField` and `config` into `SkillsSection` |

---

## Task 1: Config Model — Add `ToolsConfig`

**Files:**
- Modify: `config/config.go`

- [x] **Step 1: Add `ToolsConfig` struct and wire into `Config`**

Insert the new struct right after `SkillsConfig` (around line 120), then add the field to `Config`:

```go
// ToolsConfig records user tool enable/disable selections.
// An empty struct means "all registered tools are active".
type ToolsConfig struct {
	Disabled []string `yaml:"disabled,omitempty"`
}
```

In the `Config` struct, add after the `Skills` field (around line 60):

```go
	Skills            SkillsConfig              `yaml:"skills,omitempty"`
	Tools             ToolsConfig               `yaml:"tools,omitempty"`
```

- [x] **Step 2: Verify Go compiles**

Run: `go build ./config`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add config/config.go
git commit -m "config: add ToolsConfig and Config.Tools field"
```

---

## Task 2: Config Descriptor — Register `tools` Section

**Files:**
- Create: `config/descriptor/tools.go`

- [x] **Step 1: Create the descriptor file**

```go
package descriptor

func init() {
	Register(Section{
		Key:     "tools",
		Label:   "Tools",
		Summary: "Enable or disable system tools. Disabled tools are hidden from the LLM.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "disabled",
				Label: "Disabled tools",
				Help:  "Tools listed here are not exposed to the LLM.",
				Kind:  FieldMultiSelect,
			},
		},
	})
}
```

- [x] **Step 2: Verify Go compiles**

Run: `go build ./config/descriptor`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add config/descriptor/tools.go
git commit -m "config/descriptor: register tools section descriptor"
```

---

## Task 3: API DTO — Extend `ToolDTO`

**Files:**
- Modify: `api/dto.go`

- [x] **Step 1: Extend `ToolDTO`**

Replace the existing `ToolDTO` struct (around line 120):

```go
// ToolDTO describes a single tool exposed by /api/tools.
type ToolDTO struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Toolset     string `json:"toolset,omitempty"`
	Enabled     bool   `json:"enabled"`
}
```

- [x] **Step 2: Verify Go compiles**

Run: `go build ./api`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add api/dto.go
git commit -m "api: extend ToolDTO with Toolset and Enabled"
```

---

## Task 4: Server — Add `disabledTools()` and `activeToolReg()`

**Files:**
- Modify: `api/server.go`

- [x] **Step 1: Add import for `sort`**

Add `"sort"` to the imports at the top of `api/server.go`.

- [x] **Step 2: Add helper methods after `currentDeps()`**

Insert after the `currentDeps()` method (around line 195):

```go
// disabledTools returns a set of tool names that are currently disabled
// according to the live config. Since s.opts.Config is atomically
// updated by handleConfigPut, this always reflects the latest state.
func (s *Server) disabledTools() map[string]bool {
	m := make(map[string]bool)
	for _, name := range s.opts.Config.Tools.Disabled {
		m[name] = true
	}
	return m
}

// activeToolReg returns a new Registry containing only the tools that
// are NOT in the disabled list. Callers get a fresh copy on each call
// so the result is safe to pass into the engine. The overhead is
// negligible because registries are small (< 50 entries).
func (s *Server) activeToolReg() *tool.Registry {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		return nil
	}
	disabled := s.disabledTools()
	active := tool.NewRegistry()
	for _, e := range deps.ToolReg.Entries(nil) {
		if !disabled[e.Name] {
			active.Register(e)
		}
	}
	return active
}
```

- [x] **Step 3: Update `RunTurn` to use `activeToolReg()`**

Find the engine construction in `RunTurn` (around line 270):

```go
	eng := agent.NewEngineWithToolsAndAux(
		deps.Provider, deps.AuxProvider, deps.Storage,
		deps.ToolReg, deps.AgentCfg, deps.Platform,
	)
```

Replace with:

```go
	eng := agent.NewEngineWithToolsAndAux(
		deps.Provider, deps.AuxProvider, deps.Storage,
		s.activeToolReg(), deps.AgentCfg, deps.Platform,
	)
```

- [x] **Step 4: Verify Go compiles**

Run: `go build ./api`
Expected: no errors

- [x] **Step 5: Commit**

```bash
git add api/server.go
git commit -m "api: add disabledTools and activeToolReg helpers; use filtered registry in RunTurn"
```

---

## Task 5: Tools Handler — Implement `handleToolsList`

**Files:**
- Modify: `api/handlers_tools.go`

- [x] **Step 1: Rewrite the stub handler**

Replace the entire file content:

```go
package api

import (
	"net/http"
	"sort"
)

// handleToolsList responds to GET /api/tools with all registered tools
// and their current enabled/disabled status. The full registry is
// always exposed so the UI can list disabled tools for re-enabling.
func (s *Server) handleToolsList(w http.ResponseWriter, _ *http.Request) {
	deps := s.currentDeps()
	if deps.ToolReg == nil {
		writeJSON(w, ToolsResponse{Tools: []ToolDTO{}})
		return
	}

	disabled := s.disabledTools()
	entries := deps.ToolReg.Entries(nil)
	out := make([]ToolDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, ToolDTO{
			Name:        e.Name,
			Description: e.Description,
			Toolset:     e.Toolset,
			Enabled:     !disabled[e.Name],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, ToolsResponse{Tools: out})
}
```

- [x] **Step 2: Verify Go compiles**

Run: `go build ./api`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add api/handlers_tools.go
git commit -m "api: implement handleToolsList returning real tool list with enabled state"
```

---

## Task 6: Conversation Handler — Use `activeToolReg()`

**Files:**
- Modify: `api/handlers_conversation.go`

- [x] **Step 1: Update engine construction in `handleConversationPost`**

Find the engine construction (around line 165):

```go
	eng := agent.NewEngineWithToolsAndAux(
		s.currentDeps().Provider, s.currentDeps().AuxProvider, s.currentDeps().Storage,
		s.currentDeps().ToolReg, s.currentDeps().AgentCfg, s.currentDeps().Platform,
	)
```

Replace with:

```go
	eng := agent.NewEngineWithToolsAndAux(
		s.currentDeps().Provider, s.currentDeps().AuxProvider, s.currentDeps().Storage,
		s.activeToolReg(), s.currentDeps().AgentCfg, s.currentDeps().Platform,
	)
```

- [x] **Step 2: Verify Go compiles**

Run: `go build ./api`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add api/handlers_conversation.go
git commit -m "api: use activeToolReg in handleConversationPost"
```

---

## Task 7: Backend Tests — Tool Listing and Filtering

**Files:**
- Create: `api/handlers_tools_test.go`

- [x] **Step 1: Create the test file**

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/odysseythink/hermind/config"
	"github.com/odysseythink/hermind/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolsList_EmptyRegistry(t *testing.T) {
	s := newTestServer(t)
	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/tools", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Empty(t, resp.Tools)
}

func TestToolsList_WithRegistry(t *testing.T) {
	cfg := &config.Config{}
	s, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "alpha", Description: "Alpha tool", Toolset: "file"},
				{Name: "beta", Description: "Beta tool", Toolset: "web"},
			}),
		},
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/tools", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Tools, 2)
	assert.Equal(t, "alpha", resp.Tools[0].Name)
	assert.Equal(t, "Alpha tool", resp.Tools[0].Description)
	assert.Equal(t, "file", resp.Tools[0].Toolset)
	assert.True(t, resp.Tools[0].Enabled)
	assert.Equal(t, "beta", resp.Tools[1].Name)
	assert.True(t, resp.Tools[1].Enabled)
}

func TestToolsList_DisabledTools(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{Disabled: []string{"beta"}},
	}
	s, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "alpha", Description: "Alpha tool"},
				{Name: "beta", Description: "Beta tool"},
				{Name: "gamma", Description: "Gamma tool"},
			}),
		},
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	s.Router().ServeHTTP(rr, httptest.NewRequest("GET", "/api/tools", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var resp ToolsResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.Len(t, resp.Tools, 3)

	byName := make(map[string]ToolDTO)
	for _, t := range resp.Tools {
		byName[t.Name] = t
	}
	assert.True(t, byName["alpha"].Enabled)
	assert.False(t, byName["beta"].Enabled)
	assert.True(t, byName["gamma"].Enabled)
}

func TestActiveToolReg_FiltersDisabled(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{Disabled: []string{"beta"}},
	}
	s, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "alpha"},
				{Name: "beta"},
				{Name: "gamma"},
			}),
		},
	})
	require.NoError(t, err)

	active := s.activeToolReg()
	require.NotNil(t, active)
	names := []string{}
	for _, e := range active.Entries(nil) {
		names = append(names, e.Name)
	}
	assert.Equal(t, []string{"alpha", "gamma"}, names)
}

func TestActiveToolReg_AllDisabled(t *testing.T) {
	cfg := &config.Config{
		Tools: config.ToolsConfig{Disabled: []string{"alpha", "beta"}},
	}
	s, err := NewServer(&ServerOpts{
		Config: cfg,
		Deps: &EngineDeps{
			ToolReg: buildTestRegistry(t, []testTool{
				{Name: "alpha"},
				{Name: "beta"},
			}),
		},
	})
	require.NoError(t, err)

	active := s.activeToolReg()
	require.NotNil(t, active)
	assert.Empty(t, active.Entries(nil))
}

func TestActiveToolReg_NilRegistry(t *testing.T) {
	s := newTestServer(t)
	assert.Nil(t, s.activeToolReg())
}

type testTool struct {
	Name        string
	Description string
	Toolset     string
}

func buildTestRegistry(t *testing.T, tools []testTool) *tool.Registry {
	t.Helper()
	reg := tool.NewRegistry()
	for _, tt := range tools {
		reg.Register(tool.Entry{
			Name:        tt.Name,
			Description: tt.Description,
			Toolset:     tt.Toolset,
			Handler:     func(_ context.Context, _ any) (any, error) { return nil, nil },
		})
	}
	return reg
}
```

- [x] **Step 2: Run the new tests**

Run: `go test ./api -run TestToolsList -v`
Expected: PASS for all 3 TestToolsList tests

Run: `go test ./api -run TestActiveToolReg -v`
Expected: PASS for all 3 TestActiveToolReg tests

- [x] **Step 3: Run full api test suite**

Run: `go test ./api -count=1`
Expected: all tests pass (pre-existing failures unrelated to this change remain)

- [x] **Step 4: Commit**

```bash
git add api/handlers_tools_test.go
git commit -m "api: add tests for tool listing and active registry filtering"
```

---

## Task 8: Frontend Schema — Extend `ToolSchema`

**Files:**
- Modify: `web/src/api/schemas.ts`

- [x] **Step 1: Extend `ToolSchema`**

Find the existing `ToolSchema` (around line 158) and replace:

```ts
export const ToolSchema = z.object({
  name: z.string(),
  description: z.string().optional(),
  toolset: z.string().optional(),
  enabled: z.boolean(),
});
export type Tool = z.infer<typeof ToolSchema>;
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `cd web && pnpm tsc --noEmit`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add web/src/api/schemas.ts
git commit -m "web: extend ToolSchema with toolset and enabled fields"
```

---

## Task 9: Frontend — `SkillsSection` Tool Panel

**Files:**
- Modify: `web/src/components/groups/skills/SkillsSection.tsx`
- Modify: `web/src/components/groups/skills/SkillsSection.module.css`

- [x] **Step 1: Extend `SkillsSectionProps` with `onSectionField`**

Add the optional callback to the interface (around line 12):

```tsx
export interface SkillsSectionProps {
  section: ConfigSectionT;
  value: Record<string, unknown>;
  originalValue: Record<string, unknown>;
  onField: (field: string, value: unknown) => void;
  onSectionField?: (sectionKey: string, field: string, value: unknown) => void;
  config?: Record<string, unknown>;
}
```

- [x] **Step 2: Add imports**

Add `ToolsResponseSchema` to the schemas import (line 6):

```tsx
import { SkillsResponseSchema, ToolsResponseSchema, type ConfigSection as ConfigSectionT } from '../../../api/schemas';
```

- [x] **Step 3: Add tool panel state types**

Add after the `FetchState` type (around line 27):

```tsx
type ToolFetchState =
  | { status: 'loading' }
  | { status: 'ok'; tools: { name: string; description: string; toolset: string; enabled: boolean }[] }
  | { status: 'error'; message: string };
```

- [x] **Step 4: Add tool state and fetch logic**

Inside the component, add after the skills fetch state:

```tsx
  const [toolState, setToolState] = useState<ToolFetchState>({ status: 'loading' });

  const loadTools = useCallback((signal?: AbortSignal) => {
    setToolState({ status: 'loading' });
    apiFetch('/api/tools', { schema: ToolsResponseSchema, signal })
      .then(resp => {
        const normalized = resp.tools.map(t => ({
          name: t.name,
          description: t.description || '',
          toolset: t.toolset || '',
          enabled: t.enabled,
        }));
        setToolState({ status: 'ok', tools: normalized });
      })
      .catch(err => {
        if (signal?.aborted) return;
        const msg = err instanceof ApiError ? `${err.status}` : err instanceof Error ? err.message : String(err);
        setToolState({ status: 'error', message: msg });
      });
  }, []);
```

Update the `useEffect` to also fetch tools:

```tsx
  useEffect(() => {
    const ac = new AbortController();
    load(ac.signal);
    loadTools(ac.signal);
    return () => ac.abort();
  }, [load, loadTools]);
```

- [x] **Step 5: Add toggle helper and tool rows computation**

After `toggleSkill`, add:

```tsx
  function toggleTool(name: string, nextEnabled: boolean) {
    if (!props.onSectionField) return;
    const toolsCfg = (props.config?.tools as Record<string, unknown> | undefined) ?? {};
    const cur = (toolsCfg.disabled as string[] | undefined) ?? [];
    const next = nextEnabled
      ? cur.filter(n => n !== name)
      : [...cur.filter(n => n !== name), name].sort();
    props.onSectionField('tools', 'disabled', next);
  }
```

After the skills `rows` computation, add tool rows:

```tsx
  const toolRows = toolState.status === 'ok'
    ? toolState.tools.map(t => ({
        name: t.name,
        description: t.description,
        toolset: t.toolset,
        enabled: t.enabled,
      })).sort((a, b) => a.name.localeCompare(b.name))
    : [];
  const toolDisabledCount = toolRows.filter(r => !r.enabled).length;
```

- [x] **Step 6: Add the tool panel UI**

After the skills list panel (before the closing `</section>`), insert:

```tsx
      <div className={styles.panel}>
        <h3 className={styles.panelTitle}>
          {t('tools.list')}
          <span className={styles.panelTitleMeta}>
            — {t('tools.listMeta', { count: toolRows.length, disabledCount: toolDisabledCount })}
          </span>
        </h3>

        {toolState.status === 'loading' && (
          <div className={styles.statusRow}>{t('tools.loading')}</div>
        )}
        {toolState.status === 'error' && (
          <div className={styles.errorRow}>
            <span>{t('tools.error', { msg: toolState.message })}</span>
            <button type="button" className={styles.retryButton} onClick={() => loadTools()}>
              {t('tools.errorRetry')}
            </button>
          </div>
        )}
        {toolState.status === 'ok' && toolRows.length === 0 && (
          <div className={styles.statusRow}>{t('tools.empty')}</div>
        )}
        {toolState.status === 'ok' && toolRows.map(row => (
          <div key={row.name} className={styles.skillRow}>
            <div>
              <div>
                <span className={styles.skillName}>{row.name}</span>
                {row.toolset && (
                  <span className={styles.skillNameMissing}>{row.toolset}</span>
                )}
              </div>
              {row.description && <div className={styles.skillDescription}>{row.description}</div>}
            </div>
            <Switch
              checked={row.enabled}
              onChange={(next) => toggleTool(row.name, next)}
              ariaLabel={t('tools.toggleAria', { name: row.name })}
            />
          </div>
        ))}
      </div>
```

- [x] **Step 7: Add CSS styles for toolset tag**

Add to `SkillsSection.module.css`:

```css
.skillNameMissing {
  margin-left: 0.5rem;
  font-size: 0.75rem;
  color: var(--text-tertiary, #888);
}
```

(If `.skillNameMissing` already exists, verify it has the right styling. The existing class is used for "missing" skills; reusing it for toolset tags is acceptable since the visual intent — a small muted label next to the name — is the same.)

- [x] **Step 8: Add translation keys**

In the i18n resources (check `web/src/i18n/` for the `ui` namespace), add these keys:

```json
{
  "tools.list": "Tools",
  "tools.listMeta": "{{count}} total, {{disabledCount}} disabled",
  "tools.loading": "Loading tools...",
  "tools.error": "Failed to load tools: {{msg}}",
  "tools.errorRetry": "Retry",
  "tools.empty": "No tools registered.",
  "tools.toggleAria": "Enable {{name}}"
}
```

If the project uses a fixture/generator for translations, run the generator. Otherwise add the keys to the relevant translation file(s).

- [x] **Step 9: Verify TypeScript compiles**

Run: `cd web && pnpm tsc --noEmit`
Expected: no errors

- [x] **Step 10: Commit**

```bash
git add web/src/components/groups/skills/SkillsSection.tsx web/src/components/groups/skills/SkillsSection.module.css
git commit -m "web: add tool panel to SkillsSection with fetch, toggle, and UI"
```

---

## Task 10: Frontend — `SettingsPanel` Pass `onSectionField`

**Files:**
- Modify: `web/src/components/shell/SettingsPanel.tsx`

- [x] **Step 1: Pass the new props to `SkillsSection`**

Find the `SkillsSection` render (around line 110) and replace:

```tsx
        <SkillsSection
          section={section}
          value={value ?? {}}
          originalValue={original ?? {}}
          onField={(field, v) => props.onConfigField(section.key, field, v)}
          config={props.config as unknown as Record<string, unknown>}
        />
```

With:

```tsx
        <SkillsSection
          section={section}
          value={value ?? {}}
          originalValue={original ?? {}}
          onField={(field, v) => props.onConfigField(section.key, field, v)}
          onSectionField={props.onConfigField}
          config={props.config as unknown as Record<string, unknown>}
        />
```

- [x] **Step 2: Verify TypeScript compiles**

Run: `cd web && pnpm tsc --noEmit`
Expected: no errors

- [x] **Step 3: Commit**

```bash
git add web/src/components/shell/SettingsPanel.tsx
git commit -m "web: pass onSectionField and config to SkillsSection"
```

---

## Task 11: Frontend Tests — SkillsSection Tool Panel

**Files:**
- Modify: `web/src/components/groups/skills/SkillsSection.test.tsx`

- [x] **Step 1: Add mock helper for tools API**

After `mockSkillsApi`, add:

```ts
function mockToolsApi(tools: Array<{ name: string; description?: string; toolset?: string; enabled: boolean }>) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response(JSON.stringify({ tools }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
}
```

- [x] **Step 2: Add tool panel rendering test**

Add inside the `describe('SkillsSection')` block:

```ts
  it('renders tool panel with fetched tools and switches', async () => {
    mockSkillsApi([]);
    mockToolsApi([
      { name: 'file_read', description: 'Read a file', toolset: 'file', enabled: true },
      { name: 'web_search', description: 'Search the web', toolset: 'web', enabled: false },
    ]);
    render(
      <SkillsSection
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        onSectionField={vi.fn()}
        config={{ tools: { disabled: ['web_search'] } }}
      />,
    );
    await waitFor(() => screen.getByText('file_read'));
    expect(screen.getByText('file_read')).toBeInTheDocument();
    expect(screen.getByText('Read a file')).toBeInTheDocument();
    expect(screen.getByText('web_search')).toBeInTheDocument();

    const switches = screen.getAllByRole('switch');
    // First switch is auto_extract, remaining are skill/tool switches
    const toolSwitches = switches.filter(s => /^Enable /.test(s.getAttribute('aria-label') ?? ''));
    expect(toolSwitches.length).toBeGreaterThanOrEqual(2);
  });

  it('toggling a tool OFF dispatches onSectionField with updated disabled list', async () => {
    mockSkillsApi([]);
    mockToolsApi([
      { name: 'file_read', description: '', toolset: 'file', enabled: true },
      { name: 'web_search', description: '', toolset: 'web', enabled: true },
    ]);
    const onSectionField = vi.fn();
    render(
      <SkillsSection
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        onSectionField={onSectionField}
        config={{ tools: { disabled: [] } }}
      />,
    );
    await waitFor(() => screen.getByText('file_read'));
    fireEvent.click(screen.getByRole('switch', { name: 'Enable file_read' }));
    expect(onSectionField).toHaveBeenCalledWith('tools', 'disabled', ['file_read']);
  });

  it('toggling a disabled tool ON dispatches onSectionField with removed name', async () => {
    mockSkillsApi([]);
    mockToolsApi([
      { name: 'file_read', description: '', toolset: 'file', enabled: false },
      { name: 'web_search', description: '', toolset: 'web', enabled: true },
    ]);
    const onSectionField = vi.fn();
    render(
      <SkillsSection
        section={skillsSection}
        value={{}}
        originalValue={{}}
        onField={vi.fn()}
        onSectionField={onSectionField}
        config={{ tools: { disabled: ['file_read'] } }}
      />,
    );
    await waitFor(() => screen.getByText('file_read'));
    fireEvent.click(screen.getByRole('switch', { name: 'Enable file_read' }));
    expect(onSectionField).toHaveBeenCalledWith('tools', 'disabled', []);
  });
```

- [x] **Step 3: Run frontend tests**

Run: `cd web && pnpm test -- SkillsSection.test.tsx`
Expected: all tests pass

- [x] **Step 4: Commit**

```bash
git add web/src/components/groups/skills/SkillsSection.test.tsx
git commit -m "web: add tests for SkillsSection tool panel"
```

---

## Task 12: Full Verification

- [x] **Step 1: Backend tests**

Run: `go test ./... -count=1`
Expected: all tests that were passing before continue to pass; new tests pass

- [x] **Step 2: Frontend type check**

Run: `cd web && pnpm tsc --noEmit`
Expected: no type errors

- [x] **Step 3: Frontend build**

Run: `cd web && pnpm build`
Expected: build succeeds

- [x] **Step 4: Frontend tests**

Run: `cd web && pnpm test`
Expected: all tests pass

- [x] **Step 5: Integration sanity — start server and hit endpoints**

Start the server (or use an existing test harness) and verify:
1. `GET /api/tools` returns a sorted list with `enabled` fields
2. `GET /api/config/schema` includes the `tools` section under `group_id: "skills"`
3. `PUT /api/config` with `tools.disabled: ["some_tool"]` persists and `GET /api/tools` reflects the change

- [x] **Step 6: Commit final verification (optional)**

If any fix-up commits were needed during verification, squash or keep as-is depending on project conventions.

---

## Self-Review Checklist

### 1. Spec Coverage

| Design Requirement | Task |
|---|---|
| `tools.disabled` config field | Task 1 |
| `tools` descriptor (GroupID="skills") | Task 2 |
| `GET /api/tools` returns real list + enabled status | Task 5 |
| `activeToolReg()` filters disabled tools | Task 4 |
| `RunTurn` uses filtered registry | Task 4 |
| `handleConversationPost` uses filtered registry | Task 6 |
| `SkillsSection` fetches and renders tools | Task 9 |
| `SkillsSection` toggles dispatch `onSectionField` | Task 9 |
| `SettingsPanel` passes `onSectionField` | Task 10 |
| Backend tests for tool listing/filtering | Task 7 |
| Frontend tests for tool panel | Task 11 |
| Backward compatible (empty = all enabled) | Implicit — empty `ToolsConfig` means no filtering |
| `ToolReg` stays complete for API listing | Explicit — `activeToolReg()` copies from full `deps.ToolReg` |

**Gap: none.**

### 2. Placeholder Scan

- No "TBD", "TODO", "implement later", "fill in details"
- No vague "add appropriate error handling"
- No "write tests for the above" without test code
- No "Similar to Task N" references
- All code blocks contain actual code

### 3. Type Consistency

- `ToolDTO` fields: `Name`, `Description`, `Toolset`, `Enabled` — consistent across dto.go, handlers_tools.go, handlers_tools_test.go, and schemas.ts
- `activeToolReg()` returns `*tool.Registry` — matches `agent.NewEngineWithToolsAndAux` signature
- `onSectionField` signature: `(sectionKey: string, field: string, value: unknown) => void` — consistent between props definition and SettingsPanel usage
- `ToolsConfig.Disabled` is `[]string` — consistent between config.go, server.go disabledTools(), and frontend toggle logic

---

## Execution Handoff

**Plan complete and saved to `.gpowers/plans/2026-05-18-tools-toggle-in-skills-page.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints for review.

Which approach?
