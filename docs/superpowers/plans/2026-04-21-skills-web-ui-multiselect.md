# Skills Web UI — Multi-Select Field Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the Skills sidebar group in the web UI so it renders a discovered-skills checkbox list instead of the current "coming soon" fallback. Toggling a checkbox writes back to `config.skills.disabled` via the existing `PUT /api/config` path.

**Architecture:** Extend the generic config descriptor system with a new `FieldMultiSelect` field kind whose storage is `[]string` and whose choices are an enum. Register a `skills` section (shape=`ShapeMap`, one `disabled` field of this new kind). The schema handler enriches that field's `Enum` at response time by running the existing `skills.Loader` against `skills.DefaultHome()`, so the UI receives all discovered skill names without any new endpoint. A new `MultiSelectField` React component renders a checkbox-per-choice inside `ConfigSection`, using the existing `setPath`/`edit/config-field` state plumbing to round-trip the `[]string` value through `PUT /api/config`. The `platform_disabled` map stays out of scope — it's a later layered concern, not a blocker for "UI no longer empty".

**Tech Stack:** Go 1.21+, React + TypeScript, zod (schema validation), Vitest + React Testing Library, existing `config/descriptor/` + `api/` + `web/src/components/` packages.

**Spec:** derived from the conversation on 2026-04-21; no separate spec file. Inputs: `docs/superpowers/plans/2026-04-17-skills-config-persistence.md` (completed — established `config.SkillsConfig`, persistence, CLI enable/disable), `config/descriptor/memory.go` (reference descriptor), `web/src/components/groups/gateway/` (reference bespoke panel, NOT used here — we stay descriptor-driven).

**Scope note — what this does NOT ship:**
- **No `platform_disabled` editing.** Only global `skills.disabled` is editable in this plan. Per-platform overrides (CLI / gateway / cron) require a different UI affordance and are deferred.
- **No `/api/skills/*` mutation endpoints.** The Skills UI piggybacks on the existing `GET /api/config` + `PUT /api/config` flow; the older stub handler `api/handlers_skills.go` is left alone (CLI `hermind skills enable/disable` still writes the same config key from a different path — both paths converge on the same YAML).
- **No live skill catalog refresh.** The UI reads the catalog once per page load via `/api/config/schema`. Installing a new skill requires a page reload. Live refresh can wait.
- **No descriptor-wide generalization of "dynamic enum source".** The skills loader is wired inline in `handleConfigSchema` behind a tiny helper. If a second descriptor needs dynamic enum values, extract then.
- **No removal of the old `api/handlers_skills.go`.** That handler still powers whatever CLI-facing paths consume `/api/skills`. Leaving it keeps blast radius small.
- **No rebuild-the-webroot-bundle in CI.** The smoke task at the end walks the local `pnpm build` + commit-the-bundle flow the rest of this repo uses.

---

## File Structure

**Modified (Go):**
- `config/descriptor/descriptor.go` — add `FieldMultiSelect` to the `FieldKind` enum (const + `String()` case).
- `config/descriptor/descriptor_test.go` — extend the invariants/String tests to cover `FieldMultiSelect`.
- `api/handlers_config_schema.go` — after building the sections slice, enrich the `skills.disabled` field's `Enum` with discovered skill names via `skills.NewLoader(skills.DefaultHome()).Load()`.
- `api/handlers_config_schema_test.go` — new test that seeds a fake `HERMIND_HOME` with two SKILL.md files, calls `/api/config/schema`, asserts the `skills` section exists and `disabled.Enum` contains both names.

**Created (Go):**
- `config/descriptor/skills.go` — `init()` that registers the `skills` section with one `FieldMultiSelect` field named `disabled`.
- `config/descriptor/skills_test.go` — verifies registration, GroupID, Shape, and the single-field `disabled` contract.

**Modified (TypeScript):**
- `web/src/api/schemas.ts` — extend `ConfigFieldKindSchema` with `'multiselect'`.
- `web/src/api/schemas.test.ts` — add a parse test for `kind: 'multiselect'`.
- `web/src/components/ConfigSection.tsx` — add a `case 'multiselect':` branch that reads the value as `string[]` and dispatches to a new `MultiSelectField`.
- `web/src/shell/groups.ts` — flip the Skills group from `plannedStage: '6'` to `plannedStage: 'done'`.
- `web/src/shell/groups.test.ts` — update the assertion that matched `'6'` (if any).

**Created (TypeScript):**
- `web/src/components/fields/MultiSelectField.tsx` — new component; checkbox-per-enum-choice, stores a deduped sorted `string[]`.
- `web/src/components/fields/MultiSelectField.test.tsx` — three tests (render, toggle-on, toggle-off).

**Modified (docs):**
- `docs/smoke/skills.md` — new smoke flow (create file).

**Webroot rebuild (commit-tracked artifact):**
- `api/webroot/assets/index-*.js` + `index.html` — produced by `pnpm --dir web build`, committed in Task 8.

---

## Task 1: Backend — add `FieldMultiSelect` kind

**Files:**
- Modify: `config/descriptor/descriptor.go`
- Modify: `config/descriptor/descriptor_test.go`

- [ ] **Step 1: Read the existing descriptor_test.go to locate the `FieldKind.String()` test**

Run:
```
grep -n 'FieldKind\|String\(\)' config/descriptor/descriptor_test.go
```
Expected: there's at least one table-style test checking each `FieldKind` maps to the right string. Find it. If no such test exists, treat Step 2 as creating it from scratch.

- [ ] **Step 2: Write the failing test**

Append to `config/descriptor/descriptor_test.go`:

```go
func TestFieldKindMultiSelectString(t *testing.T) {
	if got := FieldMultiSelect.String(); got != "multiselect" {
		t.Errorf("FieldMultiSelect.String() = %q, want %q", got, "multiselect")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./config/descriptor/ -run TestFieldKindMultiSelectString -v`
Expected: FAIL to compile — `FieldMultiSelect` undefined.

- [ ] **Step 4: Add the constant and String() case**

Edit `config/descriptor/descriptor.go`. In the `const (...)` block after `FieldFloat`, add:

```go
	FieldMultiSelect
```

In the `String()` switch, add the case before the default return:

```go
	case FieldMultiSelect:
		return "multiselect"
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./config/descriptor/ -run TestFieldKindMultiSelectString -v`
Expected: PASS.

- [ ] **Step 6: Run the whole descriptor package to confirm no regressions**

Run: `go test ./config/descriptor/...`
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add config/descriptor/descriptor.go config/descriptor/descriptor_test.go
git commit -m "feat(descriptor): FieldMultiSelect kind"
```

---

## Task 2: Backend — register the `skills` section

**Files:**
- Create: `config/descriptor/skills.go`
- Create: `config/descriptor/skills_test.go`

- [ ] **Step 1: Write the failing test**

Create `config/descriptor/skills_test.go`:

```go
package descriptor

import "testing"

func TestSkillsSectionRegistered(t *testing.T) {
	s, ok := Get("skills")
	if !ok {
		t.Fatalf("Get(\"skills\") returned ok=false — did skills.go init() register?")
	}
	if s.GroupID != "skills" {
		t.Errorf("GroupID = %q, want %q", s.GroupID, "skills")
	}
	if s.Shape != ShapeMap {
		t.Errorf("Shape = %v, want ShapeMap", s.Shape)
	}
	if s.Label == "" {
		t.Error("Label is empty")
	}
}

func TestSkillsSectionHasDisabledMultiSelectField(t *testing.T) {
	s, _ := Get("skills")
	if len(s.Fields) != 1 {
		t.Fatalf("expected exactly 1 field, got %d: %+v", len(s.Fields), s.Fields)
	}
	f := s.Fields[0]
	if f.Name != "disabled" {
		t.Errorf("field name = %q, want %q", f.Name, "disabled")
	}
	if f.Kind != FieldMultiSelect {
		t.Errorf("field kind = %s, want multiselect", f.Kind)
	}
	if f.Required {
		t.Errorf("field.Required = true, want false (empty disabled list means all enabled)")
	}
	if f.Help == "" {
		t.Errorf("field.Help is empty; users need a hint about what this does")
	}
	// Enum is left empty at descriptor registration time; handler enriches
	// it from the skills loader before emitting the schema DTO.
	if len(f.Enum) != 0 {
		t.Errorf("field.Enum should be empty at registration time (got %v); "+
			"runtime enrichment via handlers_config_schema.go supplies choices", f.Enum)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./config/descriptor/ -run TestSkillsSection -v`
Expected: FAIL — both tests fail, first one with `Get("skills") returned ok=false`.

- [ ] **Step 3: Create the descriptor file**

Create `config/descriptor/skills.go`:

```go
package descriptor

// Skills mirrors config.SkillsConfig's global `disabled` list. Choices
// are not baked into the descriptor: api/handlers_config_schema.go
// populates field.Enum at response time by walking the skills home
// directory, so newly-installed skills show up after a page reload
// without a rebuild.
//
// `platform_disabled` (per-platform override map) is deliberately
// excluded from this section — the UI affordance for editing a nested
// map-of-lists is a separate design and not blocking the "skills group
// no longer empty" goal.
func init() {
	Register(Section{
		Key:     "skills",
		Label:   "Skills",
		Summary: "Enable or disable skills across every platform. Unchecked = enabled.",
		GroupID: "skills",
		Shape:   ShapeMap,
		Fields: []FieldSpec{
			{
				Name:  "disabled",
				Label: "Disabled skills",
				Help:  "Skills listed here never activate. Check a skill to disable it globally. Install skills into $HERMIND_HOME/skills to make them appear.",
				Kind:  FieldMultiSelect,
			},
		},
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./config/descriptor/ -run TestSkillsSection -v`
Expected: both tests PASS.

- [ ] **Step 5: Run the whole descriptor package**

Run: `go test ./config/descriptor/...`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add config/descriptor/skills.go config/descriptor/skills_test.go
git commit -m "feat(descriptor): register skills section (disabled multiselect)"
```

---

## Task 3: Backend — enrich `skills.disabled.Enum` from the skill loader

**Files:**
- Modify: `api/handlers_config_schema.go`
- Modify: `api/handlers_config_schema_test.go`

- [ ] **Step 1: Write the failing test**

Append to `api/handlers_config_schema_test.go`:

```go
func TestConfigSchema_SkillsDisabledEnumFromLoader(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERMIND_HOME", dir)

	// Seed two skills under $HERMIND_HOME/skills/<category>/<name>/SKILL.md.
	seed := func(name string) {
		p := filepath.Join(dir, "skills", "demo", name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: seeded for test\n---\nbody"
		if err := os.WriteFile(filepath.Join(p, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	seed("alpha")
	seed("beta")

	srv, err := api.NewServer(&api.ServerOpts{
		Config: &config.Config{},
		Token:  "test-token",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/config/schema", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var body api.ConfigSchemaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}

	var skills *api.ConfigSectionDTO
	for i := range body.Sections {
		if body.Sections[i].Key == "skills" {
			skills = &body.Sections[i]
			break
		}
	}
	if skills == nil {
		t.Fatalf("missing skills section; got keys = %v", keysOf(body.Sections))
	}
	if skills.GroupID != "skills" {
		t.Errorf("skills.group_id = %q, want \"skills\"", skills.GroupID)
	}
	if len(skills.Fields) != 1 {
		t.Fatalf("skills.fields count = %d, want 1", len(skills.Fields))
	}
	disabled := skills.Fields[0]
	if disabled.Name != "disabled" {
		t.Errorf("field name = %q, want \"disabled\"", disabled.Name)
	}
	if disabled.Kind != "multiselect" {
		t.Errorf("field kind = %q, want \"multiselect\"", disabled.Kind)
	}

	got := map[string]bool{}
	for _, v := range disabled.Enum {
		got[v] = true
	}
	if !got["alpha"] || !got["beta"] {
		t.Errorf("disabled.Enum = %v, want to contain alpha and beta", disabled.Enum)
	}
}

// keysOf is a small helper used only by the test above to produce a
// readable failure message. If a similar helper already exists in the
// file, reuse it instead.
func keysOf(sections []api.ConfigSectionDTO) []string {
	out := make([]string, len(sections))
	for i, s := range sections {
		out[i] = s.Key
	}
	return out
}
```

Add these imports at the top of the test file if not already present: `"os"`, `"path/filepath"`. `"encoding/json"`, `"net/http"`, `"net/http/httptest"`, `"testing"`, `"github.com/odysseythink/hermind/api"`, `"github.com/odysseythink/hermind/config"` should already be present from the existing `TestConfigSchema_IncludesStorageSection`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./api/ -run TestConfigSchema_SkillsDisabledEnumFromLoader -v`
Expected: FAIL — either "missing skills section" (if Task 2's registration hasn't been linked in) OR `disabled.Enum` is empty (Task 3 enrichment not yet wired).

Common cause of the first failure: the package-level side-effect import of `config/descriptor` somewhere doesn't currently pull in the new file. Usually the `import _ "github.com/odysseythink/hermind/config/descriptor"` is already present in `api/` because of the other descriptors; the new file sits alongside them and gets picked up automatically. If `skills` is missing from `descriptor.All()`, confirm Task 2 passed.

- [ ] **Step 3: Wire the enrichment into `handleConfigSchema`**

Edit `api/handlers_config_schema.go`. Add an import for the skills package:

```go
import (
	"net/http"

	"github.com/odysseythink/hermind/config/descriptor"
	"github.com/odysseythink/hermind/skills"
)
```

Add this helper below `shapeString`:

```go
// discoveredSkillNames walks the default skills home and returns every
// discovered skill name, sorted. Returns nil on any error — a missing
// or unreadable home directory should not prevent the config schema
// response, it just means the Enum is empty (the UI then shows "no
// skills installed" naturally because there are no checkboxes to render).
func discoveredSkillNames() []string {
	l := skills.NewLoader(skills.DefaultHome())
	all, _ := l.Load()
	if len(all) == 0 {
		return nil
	}
	out := make([]string, 0, len(all))
	for _, s := range all {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}
```

Add `"sort"` to the imports (next to `"net/http"`).

Then inside `handleConfigSchema`, after the `for _, sec := range all { ... }` loop that builds `out.Sections`, add a post-pass that patches the `skills.disabled` enum:

```go
	for i := range out.Sections {
		if out.Sections[i].Key != "skills" {
			continue
		}
		names := discoveredSkillNames()
		for j := range out.Sections[i].Fields {
			if out.Sections[i].Fields[j].Name == "disabled" {
				out.Sections[i].Fields[j].Enum = names
			}
		}
	}
```

Rationale for doing it as a post-pass rather than inside the main loop: the main loop is a straight DTO transform; keeping the runtime side-effect (filesystem read) separate makes the main loop trivially re-readable and the enrichment easy to delete if skills ever becomes statically enumerated.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./api/ -run TestConfigSchema_SkillsDisabledEnumFromLoader -v`
Expected: PASS.

- [ ] **Step 5: Run the whole api package**

Run: `go test ./api/...`
Expected: all PASS. In particular, `TestConfigSchema_IncludesStorageSection` still passes (the post-pass is a no-op for non-`skills` keys).

- [ ] **Step 6: Commit**

```bash
git add api/handlers_config_schema.go api/handlers_config_schema_test.go
git commit -m "feat(api): enrich skills.disabled.Enum with discovered skills"
```

---

## Task 4: Frontend — accept `multiselect` in the zod schema

**Files:**
- Modify: `web/src/api/schemas.ts`
- Modify: `web/src/api/schemas.test.ts`

- [ ] **Step 1: Write the failing test**

Append to `web/src/api/schemas.test.ts` inside the existing `describe('ConfigFieldSchema — datalist_source', ...)` block OR a new `describe('ConfigFieldSchema — multiselect', ...)` block. Using a new describe for clarity:

```ts
describe('ConfigFieldSchema — multiselect', () => {
  it('parses kind: multiselect with enum', () => {
    const parsed = ConfigFieldSchema.parse({
      name: 'disabled',
      label: 'Disabled skills',
      kind: 'multiselect',
      enum: ['alpha', 'beta'],
    });
    expect(parsed.kind).toBe('multiselect');
    expect(parsed.enum).toEqual(['alpha', 'beta']);
  });

  it('parses kind: multiselect with empty enum (no skills installed)', () => {
    const parsed = ConfigFieldSchema.parse({
      name: 'disabled',
      label: 'Disabled skills',
      kind: 'multiselect',
    });
    expect(parsed.kind).toBe('multiselect');
    expect(parsed.enum).toBeUndefined();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir web test --run src/api/schemas.test.ts`
Expected: FAIL — zod rejects `'multiselect'` because it's not in the enum.

- [ ] **Step 3: Extend the kind enum**

Edit `web/src/api/schemas.ts`. Change:

```ts
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float',
]);
```

to:

```ts
export const ConfigFieldKindSchema = z.enum([
  'string', 'int', 'bool', 'secret', 'enum', 'float', 'multiselect',
]);
```

No other changes — `ConfigField.enum` is already optional `z.array(z.string())` which works for multiselect choices too.

- [ ] **Step 4: Run test to verify it passes**

Run: `pnpm --dir web test --run src/api/schemas.test.ts`
Expected: PASS for the new two tests and all existing ones.

- [ ] **Step 5: Commit**

```bash
git add web/src/api/schemas.ts web/src/api/schemas.test.ts
git commit -m "feat(web/schema): accept kind: multiselect on ConfigField"
```

---

## Task 5: Frontend — `MultiSelectField` component

**Files:**
- Create: `web/src/components/fields/MultiSelectField.tsx`
- Create: `web/src/components/fields/MultiSelectField.test.tsx`

- [ ] **Step 1: Write the failing test**

Create `web/src/components/fields/MultiSelectField.test.tsx`:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import MultiSelectField from './MultiSelectField';
import type { SchemaField } from '../../api/schemas';

const field: SchemaField = {
  name: 'disabled',
  label: 'Disabled skills',
  help: 'Check a skill to disable it',
  kind: 'multiselect',
  enum: ['alpha', 'beta', 'gamma'],
};

describe('MultiSelectField', () => {
  it('renders a checkbox per enum choice, reflecting initial value', () => {
    const { container } = render(
      <MultiSelectField field={field} value={['beta']} onChange={vi.fn()} />,
    );
    const boxes = container.querySelectorAll('input[type="checkbox"]');
    expect(boxes).toHaveLength(3);
    const alpha = screen.getByLabelText('alpha') as HTMLInputElement;
    const beta = screen.getByLabelText('beta') as HTMLInputElement;
    const gamma = screen.getByLabelText('gamma') as HTMLInputElement;
    expect(alpha.checked).toBe(false);
    expect(beta.checked).toBe(true);
    expect(gamma.checked).toBe(false);
  });

  it('calls onChange with the new sorted deduped array when a box is checked', () => {
    const onChange = vi.fn();
    render(<MultiSelectField field={field} value={['beta']} onChange={onChange} />);
    fireEvent.click(screen.getByLabelText('alpha'));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(['alpha', 'beta']);
  });

  it('calls onChange with the remaining array when a box is unchecked', () => {
    const onChange = vi.fn();
    render(
      <MultiSelectField
        field={field}
        value={['alpha', 'beta']}
        onChange={onChange}
      />,
    );
    fireEvent.click(screen.getByLabelText('alpha'));
    expect(onChange).toHaveBeenCalledWith(['beta']);
  });

  it('renders the empty-state hint when field.enum is empty', () => {
    const empty: SchemaField = { ...field, enum: [] };
    render(<MultiSelectField field={empty} value={[]} onChange={vi.fn()} />);
    expect(screen.getByText(/no skills/i)).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `pnpm --dir web test --run src/components/fields/MultiSelectField.test.tsx`
Expected: FAIL — import error, `MultiSelectField` module not found.

- [ ] **Step 3: Implement the component**

Create `web/src/components/fields/MultiSelectField.tsx`:

```tsx
import styles from './fields.module.css';
import type { SchemaField } from '../../api/schemas';

export interface MultiSelectFieldProps {
  field: SchemaField;
  value: string[];
  onChange: (next: string[]) => void;
}

export default function MultiSelectField({
  field,
  value,
  onChange,
}: MultiSelectFieldProps) {
  const choices = field.enum ?? [];
  const checked = new Set(value);

  if (choices.length === 0) {
    return (
      <div className={styles.row}>
        <span className={styles.label}>{field.label}</span>
        <span className={styles.help}>
          No skills installed. {field.help}
        </span>
      </div>
    );
  }

  const toggle = (name: string) => {
    const next = new Set(checked);
    if (next.has(name)) {
      next.delete(name);
    } else {
      next.add(name);
    }
    onChange(Array.from(next).sort());
  };

  return (
    <div className={styles.row}>
      <span className={styles.label}>{field.label}</span>
      <div>
        {choices.map(name => (
          <label key={name} style={{ display: 'flex', gap: '0.5rem' }}>
            <input
              type="checkbox"
              checked={checked.has(name)}
              onChange={() => toggle(name)}
              aria-label={name}
            />
            <span>{name}</span>
          </label>
        ))}
      </div>
      {field.help && <span className={styles.help}>{field.help}</span>}
    </div>
  );
}
```

Two design notes for the engineer reading this cold:
1. **Sort-on-output** keeps the persisted `disabled` list stable across toggles, which makes the YAML diff minimal and the `dirty` detection in `ConfigSection` cheap.
2. **`aria-label={name}` on the input** is what `screen.getByLabelText('alpha')` resolves — preserve this or the tests will break.

Inline style `{ display: 'flex', gap: '0.5rem' }` is a placeholder acceptable for v1; promote to `fields.module.css` (e.g. `.checkboxRow`) if a follow-up polishes styling.

- [ ] **Step 4: Run tests to verify they pass**

Run: `pnpm --dir web test --run src/components/fields/MultiSelectField.test.tsx`
Expected: all four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/fields/MultiSelectField.tsx web/src/components/fields/MultiSelectField.test.tsx
git commit -m "feat(web/fields): MultiSelectField — checkbox-per-choice renderer"
```

---

## Task 6: Frontend — dispatch `multiselect` in `ConfigSection`

**Files:**
- Modify: `web/src/components/ConfigSection.tsx`
- Add tests inline in: `web/src/components/ConfigSection.test.tsx` (create if absent; check first)

- [ ] **Step 1: Check whether `ConfigSection.test.tsx` already exists**

Run:
```
ls web/src/components/ConfigSection*
```
Expected output lists `ConfigSection.tsx` and `ConfigSection.module.css`. If `ConfigSection.test.tsx` is absent, create it in Step 2. If present, append the test case to the existing file.

- [ ] **Step 2: Write the failing test**

Append (or create the file with) this test. Skeleton for a fresh file — drop straight in if `ConfigSection.test.tsx` doesn't yet exist:

```tsx
import { describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ConfigSection from './ConfigSection';
import type { ConfigSection as ConfigSectionT } from '../api/schemas';

describe('ConfigSection — multiselect dispatch', () => {
  const section: ConfigSectionT = {
    key: 'skills',
    label: 'Skills',
    group_id: 'skills',
    shape: 'map',
    fields: [
      {
        name: 'disabled',
        label: 'Disabled skills',
        kind: 'multiselect',
        enum: ['alpha', 'beta'],
      },
    ],
  };

  it('renders MultiSelectField for kind: multiselect', () => {
    render(
      <ConfigSection
        section={section}
        value={{ disabled: ['alpha'] }}
        originalValue={{ disabled: [] }}
        onFieldChange={vi.fn()}
      />,
    );
    const alpha = screen.getByLabelText('alpha') as HTMLInputElement;
    const beta = screen.getByLabelText('beta') as HTMLInputElement;
    expect(alpha.checked).toBe(true);
    expect(beta.checked).toBe(false);
  });

  it('calls onFieldChange with the new string[] when a checkbox toggles', () => {
    const onFieldChange = vi.fn();
    render(
      <ConfigSection
        section={section}
        value={{ disabled: [] }}
        originalValue={{ disabled: [] }}
        onFieldChange={onFieldChange}
      />,
    );
    fireEvent.click(screen.getByLabelText('beta'));
    expect(onFieldChange).toHaveBeenCalledWith('disabled', ['beta']);
  });
});
```

If the file already exists, prepend the `import MultiSelectField`-related entries only if they're missing; the `describe` block can sit alongside existing `describe`s.

- [ ] **Step 3: Run test to verify it fails**

Run: `pnpm --dir web test --run src/components/ConfigSection.test.tsx`
Expected: FAIL — the component falls through to the `case 'string'/default` branch and renders a `TextInput` (which won't have the `alpha`/`beta` labels).

- [ ] **Step 4: Add the dispatch branch**

Edit `web/src/components/ConfigSection.tsx`. Import `MultiSelectField` at the top:

```ts
import MultiSelectField from './fields/MultiSelectField';
```

Inside the `switch (f.kind) { ... }`, add a `case 'multiselect':` **before** the `case 'string':` fallthrough:

```tsx
          case 'multiselect': {
            const raw = getPath(value, f.name);
            const arr = Array.isArray(raw)
              ? (raw as unknown[]).filter((x): x is string => typeof x === 'string')
              : [];
            return (
              <MultiSelectField
                key={f.name}
                field={schemaField}
                value={arr}
                onChange={(next: string[]) => onFieldChange(f.name, next)}
              />
            );
          }
```

Why the filter: `getPath` can return `unknown`. YAML loading might leave `null` inside the array if the user hand-edited config.yaml, so defensively strip non-strings rather than passing junk to the component.

- [ ] **Step 5: Run tests to verify they pass**

Run: `pnpm --dir web test --run src/components/ConfigSection.test.tsx`
Expected: all tests PASS (including any pre-existing ones).

- [ ] **Step 6: Run the whole web test suite to catch regressions in other sections**

Run: `pnpm --dir web test --run`
Expected: all suites PASS. In particular, Memory/Storage/etc sections, which use the `string`/`enum`/`secret` branches, should be unaffected.

- [ ] **Step 7: Commit**

```bash
git add web/src/components/ConfigSection.tsx web/src/components/ConfigSection.test.tsx
git commit -m "feat(web): ConfigSection dispatches multiselect to MultiSelectField"
```

---

## Task 7: Frontend — mark Skills group done

**Files:**
- Modify: `web/src/shell/groups.ts`
- Modify: `web/src/shell/groups.test.ts`

- [ ] **Step 1: Inspect the current assertion on Skills plannedStage**

Run:
```
grep -n "plannedStage\|'6'" web/src/shell/groups.test.ts
```
Expected: either a test asserts `plannedStage: '6'` for Skills (needs updating) or no assertion (nothing to update).

- [ ] **Step 2: Flip the stage**

Edit `web/src/shell/groups.ts`. Find:

```ts
  {
    id: 'skills',
    label: 'Skills',
    plannedStage: '6',
    configKeys: ['skills'],
```

Replace `plannedStage: '6'` with `plannedStage: 'done'`:

```ts
  {
    id: 'skills',
    label: 'Skills',
    plannedStage: 'done',
    configKeys: ['skills'],
```

No other field in the group changes — `bullets` and `description` still accurately describe the intent.

- [ ] **Step 3: Update the test if it pinned '6'**

If Step 1 found an assertion like `expect(...plannedStage).toBe('6')` for Skills, change `'6'` to `'done'`. If no such assertion exists, skip this step.

- [ ] **Step 4: Run the group + shell tests**

Run: `pnpm --dir web test --run src/shell/`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/shell/groups.ts web/src/shell/groups.test.ts
git commit -m "feat(web/shell): Skills group plannedStage → done"
```

---

## Task 8: Smoke — build the webroot, click through, document

**Files:**
- Create: `docs/smoke/skills.md`
- Modify: `api/webroot/index.html` + `api/webroot/assets/*` (artifact only)

- [ ] **Step 1: Rebuild the webroot bundle**

Run:
```
pnpm --dir web install   # only if web/pnpm-lock.yaml is newer than node_modules
pnpm --dir web build
```

Expected: `web/dist/` populated. Then mirror to `api/webroot/`:

```
# The repo's documented dev loop — confirm by reading web/README.md
rm -rf api/webroot/assets
cp -R web/dist/* api/webroot/
```

(If `web/README.md` specifies a different copy command or a `make webroot` target, follow that instead.)

- [ ] **Step 2: Run the full Go test suite**

Run: `go test ./...`
Expected: all PASS.

- [ ] **Step 3: Run the full web test suite**

Run: `pnpm --dir web test --run`
Expected: all PASS.

- [ ] **Step 4: Build the binary and exercise the UI**

```bash
go build -o /tmp/hermind ./cmd/hermind
export HERMIND_HOME=/tmp/hermind-skills-smoke
rm -rf "$HERMIND_HOME"
mkdir -p "$HERMIND_HOME/skills/demo/alpha"
cat > "$HERMIND_HOME/skills/demo/alpha/SKILL.md" <<'EOF'
---
name: alpha
description: smoke skill alpha
---
body
EOF
mkdir -p "$HERMIND_HOME/skills/demo/beta"
cat > "$HERMIND_HOME/skills/demo/beta/SKILL.md" <<'EOF'
---
name: beta
description: smoke skill beta
---
body
EOF
printf 'model: anthropic/claude-opus-4-6\n' > "$HERMIND_HOME/config.yaml"

/tmp/hermind web --addr 127.0.0.1:9119 --no-browser &
SERVER_PID=$!
sleep 1
```

In another shell:

```
curl -s http://127.0.0.1:9119/api/config/schema \
  -H "Authorization: Bearer $(cat $HERMIND_HOME/token)" \
  | jq '.sections[] | select(.key == "skills")'
```

Expected: a section with `"group_id": "skills"`, `"shape": "map"`, `fields[0].kind == "multiselect"`, and `fields[0].enum == ["alpha", "beta"]`.

- [ ] **Step 5: Click through the UI**

Open `http://127.0.0.1:9119/ui/` in a browser. Click the Skills tab in the sidebar. Expected:
- The main panel titled "Skills" renders.
- Two checkboxes labelled `alpha` and `beta`, both unchecked.
- Check `alpha`. Click Apply (if dirty/apply flow is present in the UI; if not, edits persist via autosave — check `cli/web.go` / existing UX conventions).
- Reload the browser. Expected: `alpha` checkbox remains checked.
- Open `$HERMIND_HOME/config.yaml`. Expected: contains
  ```yaml
  skills:
    disabled:
      - alpha
  ```

If any of these fail, halt and diagnose before proceeding. Kill the server:
```
kill $SERVER_PID
```

- [ ] **Step 6: Create the smoke doc**

Create `docs/smoke/skills.md` with:

```markdown
# Skills web UI smoke flow

Covers: `config/descriptor/skills.go`, `api/handlers_config_schema.go`
skills enrichment, `web/src/components/fields/MultiSelectField.tsx`.

## Prerequisites

- Go binary built from current branch: `go build -o /tmp/hermind ./cmd/hermind`
- A clean `HERMIND_HOME` pointed at `/tmp/hermind-skills-smoke`
- Two seeded skills under `$HERMIND_HOME/skills/demo/{alpha,beta}/SKILL.md`
- A minimal `config.yaml` containing at least `model: anthropic/claude-opus-4-6`

## Steps

1. Start hermind:
   ```
   /tmp/hermind web --addr 127.0.0.1:9119 --no-browser
   ```

2. In a second shell, dump the schema and confirm skills is present:
   ```
   curl -s http://127.0.0.1:9119/api/config/schema \
     -H "Authorization: Bearer $(cat $HERMIND_HOME/token)" \
     | jq '.sections[] | select(.key == "skills")'
   ```
   Expected: JSON with `shape: "map"`, a single `disabled` field of `kind: "multiselect"`, and `enum: ["alpha", "beta"]`.

3. In a browser, navigate to `http://127.0.0.1:9119/ui/`. Click the **Skills** tab in the sidebar.
   Expected: two checkboxes — `alpha`, `beta` — both unchecked, with the help text "Skills listed here never activate…" visible.

4. Check `alpha`, trigger the Apply workflow (click Apply or let autosave fire, matching the rest of the UI's behavior), then reload the page.
   Expected: `alpha` is still checked after reload.

5. On disk:
   ```
   grep -A2 '^skills:' $HERMIND_HOME/config.yaml
   ```
   Expected:
   ```
   skills:
     disabled:
       - alpha
   ```

6. Uncheck `alpha`, apply, reload. Expected: the `skills:` block disappears from `config.yaml` (because `Disabled []string` carries `yaml:"disabled,omitempty"` and the outer struct is likewise `omitempty`).

## Failure modes

- Skills tab still shows "coming soon": `groups.ts` not updated or the
  webroot bundle was not rebuilt + committed.
- Checkbox list is empty: the schema response's `enum` array is empty —
  either `HERMIND_HOME/skills/` is empty or `handleConfigSchema`'s
  post-pass did not run (descriptor key mismatch, most likely).
- Apply-then-reload reverts: `PUT /api/config` did not round-trip the
  `disabled` array; verify `config.SaveToPath` wrote the file and the
  UI's GET rehydration reads the same key.
```

- [ ] **Step 7: Commit the smoke doc + rebuilt webroot**

```bash
git add docs/smoke/skills.md api/webroot/
git commit -m "docs(smoke): Skills web UI smoke flow + rebuilt bundle"
```

- [ ] **Step 8: Clean up**

```
rm -rf /tmp/hermind-skills-smoke /tmp/hermind
unset HERMIND_HOME
```

---

## Completion checklist

- [ ] `go test ./...` — PASS
- [ ] `pnpm --dir web test --run` — PASS
- [ ] `go build ./cmd/hermind` — clean
- [ ] `curl /api/config/schema` shows `skills` section with `disabled.kind == "multiselect"` and non-empty `enum` when `$HERMIND_HOME/skills/` has skills
- [ ] Skills sidebar tab in the web UI renders a checkbox list, not `<ComingSoonPanel>`
- [ ] Checking a skill round-trips to `config.yaml` under `skills.disabled`
- [ ] `docs/smoke/skills.md` exists and the flow has been walked at least once
- [ ] Rebuilt webroot committed to `api/webroot/`
- [ ] `git status --short` — clean

Once all boxes are checked, the Skills group is no longer empty. Follow-up plans can:
- Add `platform_disabled` editing (per-CLI/gateway/cron overrides).
- Generalize `discoveredSkillNames()` into a descriptor-level "dynamic enum source" contract if a second section needs the same pattern.
- Live skill-catalog refresh (websocket or polling) so installing a skill shows up without page reload.
