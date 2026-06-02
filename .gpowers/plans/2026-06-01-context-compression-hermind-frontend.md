# Context Compression — Hermind Frontend UI

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a "Context Compression" section to the workspace Chat Settings page with per-workspace overrides for compression switch, threshold, and context length.

**Architecture:** Extend the existing `PUT /api/workspace/:slug/update` endpoint to accept three new compression fields (sent as strings from the frontend to support three-state boolean clearing). Build a self-contained `CompressionSettings` React component with a three-state toggle, threshold number input, and context-length override input. Wire it into `ChatSettings` alongside existing controls. Add `castToType` entries and English i18n keys.

**Tech Stack:** React 18, TailwindCSS, i18next, Go 1.26, Gin, GORM.

> **Depends on file:** `hermind-models.md` (Task M2 — `Workspace.CompressEnabled *bool`, `CompressThreshold *float64`, `CompressContextLen *int` fields must exist in the model)

---

## File Structure

| Path | Responsibility |
|---|---|
| `backend/internal/dto/workspace.go` | Extend `UpdateWorkspaceRequest` with `CompressEnabled *string`, `CompressThreshold *string`, `CompressContextLen *string` |
| `backend/internal/services/workspace_service.go` | Parse string fields into typed/nil values in `Update` method |
| `backend/internal/handlers/api_workspace_test.go` | Add behavioral test verifying compression field persistence and clearing |
| `frontend/src/pages/WorkspaceSettings/ChatSettings/CompressionSettings/index.jsx` | New React component: three-state toggle, threshold input, ctxLen input |
| `frontend/src/pages/WorkspaceSettings/ChatSettings/index.jsx` | Import and render `<CompressionSettings />` inside the form |
| `frontend/src/utils/types.js` | `castToType` definitions for compression fields |
| `frontend/src/locales/en/common.js` | English i18n keys under `chat.compression.*` |

## Dependency Overview

```
Task F0 (backend DTO + service + test)
  -> Task F1 (CompressionSettings component)
       -> Task F2 (wire into ChatSettings)
            -> Task F4 (build + manual verification)
Task F3 (castToType + i18n)  [parallel with F1/F2]
  -> Task F4
```

## Risks & Open Questions

| # | Risk | Assumption | Impact if wrong |
|---|---|---|---|
| 1 | Using `*string` in DTO for all three fields deviates from existing `*bool`/`*float64` pattern | The existing form/FormData pipeline makes it impossible to distinguish "missing" from "explicit null" with standard Gin JSON binding | DTO is slightly inconsistent; mitigation: document the rationale and encapsulate parsing in one service method |
| 2 | `workspace.compressEnabled` serialized as `null` when unset may confuse frontend default-value logic | `Workspace.GetBySlug` returns the full model JSON; `null` fields are standard in this API | Frontend must handle `null` as "follow global"; component initialization must not crash on `null` |
| 3 | i18n keys only added to `en/common.js` | i18next fallbackLng is `"en"` (verified in `i18n.js`) | Non-English users see English strings for compression UI until translators add keys |
| 4 | The web UI `UpdateWorkspace` handler currently returns `{"success":true}` while the frontend destructures `{"workspace":...}` | This is an existing API/frontend mismatch from the Go rewrite | Workspace settings pages show "Error: undefined" on save; fixing the handler response is included in Task F0 |

---

### Task F0: Extend UpdateWorkspaceRequest DTO, service, and fix handler response

**Depends on:** `hermind-models.md: Task M2` (`Workspace.CompressEnabled`, `CompressThreshold`, `CompressContextLen` fields)

**Files:**
- Modify: `backend/internal/dto/workspace.go`
- Modify: `backend/internal/services/workspace_service.go`
- Modify: `backend/internal/handlers/workspace.go`
- Test: `backend/internal/handlers/api_workspace_test.go`

- [ ] **Step 1: Write the failing test**

Add a test that updates all three compression fields, then reads the workspace back to verify persistence. Also test clearing (sending `"default"`/empty string) results in NULL in DB.

```go
func TestAPIWorkspace_UpdateCompressionFields(t *testing.T) {
	env := newAPITestEnv(t, nil)
	require.NoError(t, env.DB.Create(&models.Workspace{Name: "w", Slug: "w"}).Error)
	registerWorkspaceRoutesForTest(env)

	// Set all three fields
	payload := []byte(`{"compressEnabled":"true","compressThreshold":"0.60","compressContextLen":"64000"}`)
	req := httptest.NewRequest("POST", "/api/v1/workspace/w/update", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Workspace *models.Workspace `json:"workspace"`
		Message   string            `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Workspace)
	require.NotNil(t, body.Workspace.CompressEnabled)
	assert.True(t, *body.Workspace.CompressEnabled)
	require.NotNil(t, body.Workspace.CompressThreshold)
	assert.InDelta(t, 0.60, *body.Workspace.CompressThreshold, 0.001)
	require.NotNil(t, body.Workspace.CompressContextLen)
	assert.Equal(t, 64000, *body.Workspace.CompressContextLen)

	// Clear all three fields back to default
	payload = []byte(`{"compressEnabled":"default","compressThreshold":"","compressContextLen":""}`)
	req = httptest.NewRequest("POST", "/api/v1/workspace/w/update", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+env.APIKey)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	env.Router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var cleared models.Workspace
	require.NoError(t, env.DB.Where("slug=?", "w").First(&cleared).Error)
	assert.Nil(t, cleared.CompressEnabled)
	assert.Nil(t, cleared.CompressThreshold)
	assert.Nil(t, cleared.CompressContextLen)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/handlers/ -run TestAPIWorkspace_UpdateCompressionFields -v`
Expected: FAIL — `CompressEnabled` undefined on `models.Workspace` (if Part 2 not yet implemented) or DTO field missing.

- [ ] **Step 3: Extend DTO with string-typed compression fields**

```go
// backend/internal/dto/workspace.go

type UpdateWorkspaceRequest struct {
	Name                 string   `json:"name,omitempty"`
	OpenAiTemp           *float64 `json:"openAiTemp,omitempty"`
	OpenAiHistory        int      `json:"openAiHistory,omitempty"`
	OpenAiPrompt         *string  `json:"openAiPrompt,omitempty"`
	SimilarityThreshold  *float64 `json:"similarityThreshold,omitempty"`
	ChatProvider         *string  `json:"chatProvider,omitempty"`
	ChatModel            *string  `json:"chatModel,omitempty"`
	TopN                 *int     `json:"topN,omitempty"`
	ChatMode             *string  `json:"chatMode,omitempty"`
	QueryRefusalResponse *string  `json:"queryRefusalResponse,omitempty"`
	// Compression overrides (string-typed to support three-state clearing via FormData)
	CompressEnabled    *string `json:"compressEnabled,omitempty"`    // "true", "false", "default"
	CompressThreshold  *string `json:"compressThreshold,omitempty"`  // "0.75", "", "default"
	CompressContextLen *string `json:"compressContextLen,omitempty"` // "128000", "", "default"
}
```

Rationale: The frontend form/FormData pipeline cannot distinguish "field not present" from "explicit null" with standard Gin struct binding. Using `*string` lets the service unambiguously interpret `"default"` or `""` as "clear to NULL".

- [ ] **Step 4: Extend WorkspaceService.Update to parse and map compression fields**

In `backend/internal/services/workspace_service.go`, inside `Update`, after the existing field mappings and before `updates["last_updated_at"]`:

```go
if req.CompressEnabled != nil {
	switch *req.CompressEnabled {
	case "true":
		updates["compress_enabled"] = true
	case "false":
		updates["compress_enabled"] = false
	default:
		updates["compress_enabled"] = nil
	}
}
if req.CompressThreshold != nil {
	if *req.CompressThreshold == "" || *req.CompressThreshold == "default" {
		updates["compress_threshold"] = nil
	} else {
		if v, err := strconv.ParseFloat(*req.CompressThreshold, 64); err == nil {
			updates["compress_threshold"] = v
		}
	}
}
if req.CompressContextLen != nil {
	if *req.CompressContextLen == "" || *req.CompressContextLen == "default" {
		updates["compress_context_len"] = nil
	} else {
		if v, err := strconv.Atoi(*req.CompressContextLen); err == nil {
			updates["compress_context_len"] = v
		}
	}
}
```

Add `strconv` to imports if not already present.

- [ ] **Step 5: Fix web UI UpdateWorkspace handler response to match frontend expectations**

In `backend/internal/handlers/workspace.go`, replace the final `c.JSON(http.StatusOK, gin.H{"success": true})` in `UpdateWorkspace` with:

```go
	updated, err := h.wsSvc.GetBySlug(c.Request.Context(), ws.Slug)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"workspace": nil, "message": "Workspace updated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workspace": updated, "message": "Workspace updated"})
```

This aligns the web UI handler with the API v1 handler and the frontend's expected response shape.

- [ ] **Step 6: Verify no stale callers from the DTO change**

The `UpdateWorkspaceRequest` struct is consumed by:
- `backend/internal/handlers/workspace.go:59` (`var req dto.UpdateWorkspaceRequest`)
- `backend/internal/handlers/api_workspace.go:67` (`var req dto.UpdateWorkspaceRequest`)
- `backend/internal/services/workspace_service.go:85` (`req dto.UpdateWorkspaceRequest`)
- `backend/internal/handlers/api_workspace_test.go:80` (`payload := []byte(...)`)

Adding fields is backward-compatible; no existing caller breaks.

- [ ] **Step 7: Whole-tree typecheck + run the new test**

Run:
```bash
cd backend && go vet ./... && go test ./internal/handlers/ -run TestAPIWorkspace_UpdateCompressionFields -v
```
Expected: `go vet` passes (no stale callers); test passes.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/dto/workspace.go backend/internal/services/workspace_service.go backend/internal/handlers/workspace.go backend/internal/handlers/api_workspace_test.go
git commit -m "feat: extend workspace update API with compression field overrides"
```

---

### Task F1: Create CompressionSettings React component

**Depends on:** Task F0

**Files:**
- Create: `frontend/src/pages/WorkspaceSettings/ChatSettings/CompressionSettings/index.jsx`

- [ ] **Step 1: Write the complete component**

```jsx
import { useState } from "react";
import { useTranslation } from "react-i18next";

const ENABLED_OPTIONS = [
  { value: "default", labelKey: "chat.compression.followGlobal" },
  { value: "true", labelKey: "chat.compression.enabled" },
  { value: "false", labelKey: "chat.compression.disabled" },
];

export default function CompressionSettings({
  workspace,
  settings,
  setHasChanges,
}) {
  const { t } = useTranslation();

  // Global default from system settings (string "true" or "false")
  const globalEnabled = settings?.context_compress_enabled === "true";

  // Local state: "default" | "true" | "false"
  const [compressEnabled, setCompressEnabled] = useState(() => {
    if (workspace?.compressEnabled === true) return "true";
    if (workspace?.compressEnabled === false) return "false";
    return "default";
  });

  const [compressThreshold, setCompressThreshold] = useState(
    workspace?.compressThreshold?.toString() ?? ""
  );
  const [compressContextLen, setCompressContextLen] = useState(
    workspace?.compressContextLen?.toString() ?? ""
  );

  const isOverriding = compressEnabled !== "default";

  return (
    <div className="flex flex-col gap-y-4">
      <div className="flex flex-col">
        <label className="block input-label">{t("chat.compression.title")}</label>
        <p className="text-white text-opacity-60 text-xs font-medium py-1.5">
          {t("chat.compression.description")}
        </p>
      </div>

      {/* Three-state toggle */}
      <div className="w-fit flex gap-x-1 items-center p-1 rounded-lg bg-theme-settings-input-bg">
        <input type="hidden" name="compressEnabled" value={compressEnabled} />
        {ENABLED_OPTIONS.map((opt) => (
          <button
            key={opt.value}
            type="button"
            disabled={compressEnabled === opt.value}
            onClick={() => {
              setCompressEnabled(opt.value);
              setHasChanges(true);
            }}
            className="border-none transition-bg duration-200 px-4 py-1 text-sm text-white/60 disabled:text-white bg-transparent disabled:bg-[#687280] rounded-md hover:bg-white/10 light:hover:bg-black/10"
          >
            {t(opt.labelKey)}
          </button>
        ))}
      </div>

      {/* Global status hint */}
      {compressEnabled === "default" && (
        <p className="text-xs text-white/60">
          {t("chat.compression.globalStatus")}: {" "}
          <b>
            {globalEnabled
              ? t("chat.compression.enabled")
              : t("chat.compression.disabled")}
          </b>
        </p>
      )}

      {/* Threshold override */}
      <div className="flex flex-col gap-y-1">
        <label className="block text-xs font-medium text-white/80">
          {t("chat.compression.threshold")}
        </label>
        <p className="text-white text-opacity-60 text-xs">
          {t("chat.compression.thresholdDesc")}
        </p>
        <input
          type="hidden"
          name="compressThreshold"
          value={compressThreshold}
        />
        <input
          type="number"
          min={0.3}
          max={0.95}
          step={0.05}
          value={compressThreshold}
          onChange={(e) => {
            setCompressThreshold(e.target.value);
            setHasChanges(true);
          }}
          onWheel={(e) => e.target.blur()}
          placeholder={t("chat.compression.thresholdPlaceholder")}
          className="border-none bg-theme-settings-input-bg text-white placeholder:text-theme-settings-input-placeholder text-sm rounded-lg focus:outline-primary-button active:outline-primary-button outline-none block w-full p-2.5"
        />
      </div>

      {/* Context length override */}
      <div className="flex flex-col gap-y-1">
        <label className="block text-xs font-medium text-white/80">
          {t("chat.compression.contextLength")}
        </label>
        <p className="text-white text-opacity-60 text-xs">
          {t("chat.compression.contextLengthDesc")}
        </p>
        <input
          type="hidden"
          name="compressContextLen"
          value={compressContextLen}
        />
        <input
          type="number"
          min={1}
          step={1}
          value={compressContextLen}
          onChange={(e) => {
            setCompressContextLen(e.target.value);
            setHasChanges(true);
          }}
          onWheel={(e) => e.target.blur()}
          placeholder={t("chat.compression.contextLengthPlaceholder")}
          className="border-none bg-theme-settings-input-bg text-white placeholder:text-theme-settings-input-placeholder text-sm rounded-lg focus:outline-primary-button active:outline-primary-button outline-none block w-full p-2.5"
        />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Build to verify it compiles**

Run: `cd frontend && yarn build`
Expected: build succeeds, no JSX/type errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/WorkspaceSettings/ChatSettings/CompressionSettings/index.jsx
git commit -m "feat: add CompressionSettings workspace UI component"
```

---

### Task F2: Wire CompressionSettings into ChatSettings

**Depends on:** Task F1

**Files:**
- Modify: `frontend/src/pages/WorkspaceSettings/ChatSettings/index.jsx`

- [ ] **Step 1: Import and render the component inside the form**

Add the import at the top:
```jsx
import CompressionSettings from "./CompressionSettings";
```

Insert the component inside the `<form>` element, after `ChatTemperatureSettings` (or before — placement is flexible; place it after `ChatTemperatureSettings` for consistency):

```jsx
        <ChatTemperatureSettings
          settings={settings}
          workspace={workspace}
          setHasChanges={setHasChanges}
        />
        <CompressionSettings
          workspace={workspace}
          settings={settings}
          setHasChanges={setHasChanges}
        />
```

- [ ] **Step 2: Build to verify it compiles**

Run: `cd frontend && yarn build`
Expected: build succeeds, no import/JSX errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/WorkspaceSettings/ChatSettings/index.jsx
git commit -m "feat: wire CompressionSettings into ChatSettings page"
```

---

### Task F3: Add castToType definitions and i18n keys

**Depends on:** Task F1

**Files:**
- Modify: `frontend/src/utils/types.js`
- Modify: `frontend/src/locales/en/common.js`

- [ ] **Step 1: Add castToType entries for compression fields**

In `frontend/src/utils/types.js`, add to the `definitions` object:

```js
    compressEnabled: {
      cast: (value) => value, // "default", "true", or "false" — pass through as string
    },
    compressThreshold: {
      cast: (value) => (value === "" ? "" : value), // pass through string; empty = default
    },
    compressContextLen: {
      cast: (value) => (value === "" ? "" : value), // pass through string; empty = default
    },
```

Rationale: The backend DTO uses `*string` for these fields, so no numeric conversion is needed on the frontend. Empty string is preserved so the backend can interpret it as "clear to NULL".

- [ ] **Step 2: Add English i18n keys**

In `frontend/src/locales/en/common.js`, add under the existing `chat` namespace:

```js
  chat: {
    // ... existing keys ...
    compression: {
      title: "Context Compression",
      description:
        "Automatically compress long conversation history to stay within the model's context window.",
      followGlobal: "Follow global",
      enabled: "Enabled",
      disabled: "Disabled",
      globalStatus: "Global default",
      threshold: "Compression threshold",
      thresholdDesc:
        "Trigger compression when history exceeds this fraction of the context window. Leave empty to use path defaults (Agent 0.50, Chat 0.75).",
      thresholdPlaceholder: "0.75",
      contextLength: "Context length override",
      contextLengthDesc:
        "Override the model's context length (in tokens). Leave empty to use the built-in model map.",
      contextLengthPlaceholder: "e.g. 128000",
    },
    // ... rest of existing keys ...
  },
```

- [ ] **Step 3: Verify translation keys are well-formed**

Run: `cd frontend && node src/locales/verifyTranslations.mjs`
Expected: passes (no missing keys in `en/common.js`; other locales may report missing keys for the new entries — this is expected and acceptable because i18next falls back to English).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/utils/types.js frontend/src/locales/en/common.js
git commit -m "feat: add castToType and i18n keys for compression settings"
```

---

### Task F4: Build verification and manual end-to-end test

**Depends on:** Task F2, Task F3

**Files:** (no new files — verification only)

- [ ] **Step 1: Full frontend build**

Run: `cd frontend && yarn build`
Expected: zero errors, zero warnings. The build output should include the new component.

- [ ] **Step 2: Full backend build + test**

Run:
```bash
cd backend && go vet ./... && go test ./internal/handlers/ -run TestAPIWorkspace_UpdateCompressionFields -v
```
Expected: `go vet` passes across all packages; test passes.

- [ ] **Step 3: Manual verification**

Action:
1. Start the backend (`cd backend && make dev`) and frontend dev server (`cd frontend && yarn dev`).
2. Open a workspace's **Chat Settings** page.
3. Observe the new **Context Compression** section with three-state toggle, threshold input, and ctxLen input.
4. Select **Enabled**, set threshold to `0.65`, set ctxLen to `32000`, click **Update Workspace**.
5. Verify the network tab shows a `POST /api/workspace/:slug/update` with body containing `compressEnabled: "true"`, `compressThreshold: "0.65"`, `compressContextLen: "32000"`.
6. Reload the page and verify the saved values are restored.
7. Select **Follow global**, clear threshold and ctxLen inputs, click **Update Workspace**.
8. Verify the network tab shows `compressEnabled: "default"`, `compressThreshold: ""`, `compressContextLen: ""`.
9. Reload and verify the toggle shows **Follow global** and inputs are empty.
10. In the backend logs or via `sqlite3`, verify the `workspaces` table has `compress_enabled = 1`, `compress_threshold = 0.65`, `compress_context_len = 32000` after step 5, and all three columns are `NULL` after step 8.

Expected: all observations match the action descriptions; toast shows "Workspace updated!" on success.

- [ ] **Step 4: Commit**

```bash
# No file changes to commit for pure verification, but if any fixes were needed:
git add -A
git commit -m "fix: compression settings UI polish after manual test"
```

---

## Self-Review

Reproduce all seven as `- [ ]` checkboxes — do not shrink to five:

- [ ] **1. Spec coverage (build the table).** Map every spec section to the task that implements it.

| Design § | Requirement | Task(s) | Status |
|---|---|---|---|
| §18.4 | `UpdateWorkspaceRequest` compression fields | F0 | covered |
| §18.4 | `WorkspaceService.Update` maps compression fields | F0 | covered |
| §18.4 | Three-state toggle UI (follow global / on / off) | F1 | covered |
| §18.4 | Threshold slider/input (0.3–0.95, empty=default) | F1 | covered |
| §18.4 | ctxLen override input (empty=embedded map) | F1 | covered |
| §18.4 | Frontend workspace settings integration | F2 | covered |
| §18.4 | `castToType` for compression fields | F3 | covered |
| §18.4 | i18n keys for compression UI | F3 | covered |
| §18.4 | Backend returns updated workspace on save | F0 | covered (fixes existing mismatch) |

- [ ] **2. Placeholder scan:** Search the plan for red flags — any pattern from "No Placeholders": `TODO`/`TBD`, deferred-by-dependency excuses, and dead-code placeholders.

- [ ] **3. No phantom tasks (binary):** Every task produces a verifiable change. Zero `--allow-empty`, zero "already done in Task N" bodies. The manual verification in F4 is a real verification step, not a phantom.

- [ ] **4. Dependency soundness:** Every task's `Depends on:` is satisfied by an earlier task. F1 depends on F0 (DTO exists). F2 depends on F1 (component exists). F3 is parallel with F1/F2. F4 depends on F2 and F3.

- [ ] **5. Caller & build soundness:** Task F0 changes `UpdateWorkspaceRequest` — it verifies no stale callers via `go vet ./...`. The shared signature (DTO) is changed in exactly one task (F0). The web UI handler response is fixed in the same task.

- [ ] **6. Test-the-risk:** Task F0 includes a behavioral test that asserts DB persistence AND clearing to NULL — the state mutation is the riskiest part. The frontend UI tasks include build verification and manual verification steps.

- [ ] **7. Type consistency:** The DTO uses `*string` for all three fields; the service parses to `bool`/`float64`/`int` or `nil`; the frontend sends strings and reads `null`/`true`/`false` from the workspace JSON. Types are consistent end-to-end.
