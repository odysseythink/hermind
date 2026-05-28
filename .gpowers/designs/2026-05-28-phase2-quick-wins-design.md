# Phase 2 Quick-Wins Bundle — Design

**Date**: 2026-05-28
**Status**: Draft
**Authors**: ranwei + Claude
**Companion docs**:
- `2026-05-28-scheduled-jobs-design.md`
- `2026-05-28-memories-system-design.md`
- Source: `.gpowers/reports/server-collector-vs-go-server-gap-analysis.md` §Phase 2

## 1. Purpose

Close three "Phase 2" gaps that are either pure wiring or single-file additions, in a single bundled design. Two of the three (Filesystem Agent, Create Files Agent) are **already implemented** in the Go backend (`agent/tools/filesystem_agent.go`, `agent/tools/create_files_agent.go`, both registered in `agent/tools/builder.go`) but gated off by stub handlers. The third (Prompt History) is a small additive feature.

## 2. Scope

| Subfeature | Type | Effort |
|---|---|---|
| Filesystem Agent — flip `is-available` to honor `cfg.AgentFilesystemEnabled` | Wiring | ~30 LoC |
| Create Files Agent — flip `is-available` to honor `cfg.AgentCreateFilesEnabled` + add `pptx` via unioffice | Wiring + 1 generator | ~300 LoC |
| Prompt History — new table + hook + 3 endpoints | New feature | ~250 LoC |

### Out of scope
- pptx via hand-rolled writer (unioffice chosen instead)
- Restoring prompts via a dedicated endpoint (frontend uses existing workspace PATCH)
- Tracking non-`OpenAiPrompt` fields (agent system prompt, embed prompts, etc.)
- Pagination on prompt history (volume too small)
- Refactoring the existing single-action filesystem tool into 10 sub-tools (anything-llm shape) — current 9-action design is intentional and tested

## 3. Anything-LLM source comparison

### 3.1 Filesystem Agent
- **Node**: `server/utils/agents/aibitat/plugins/filesystem/` exposes 10 sub-skills (read_text, read_multiple, write_text, edit, create_dir, list_dir, move, copy, search, get_info). `FilesystemManager.isToolAvailable()` returns true iff `NODE_ENV=development` OR `ANYTHING_LLM_RUNTIME=docker`. The real security boundary is `validatePath()` (absolute-path + symlink resolution + allowed-dirs prefix check).
- **Go (current)**: `agent/tools/filesystem_agent.go` (332 LoC) is a single tool with an `action` discriminator covering 9 of the 10 sub-skills (no `read_multiple`). Sandboxes via `cfg.AgentFilesystemRoot` + `safeJoin`. Approval gate for destructive actions. Already registered in `builder.go:93`. The skill's `CheckFn` reads `cfg.AgentFilesystemEnabled` (default true).
- **Decision baseline**: `.gpowers/decisions/2026-05-27-fs-skill-no-docker-restriction.md` (adopted) — Hermind drops the docker-only check; the `safeJoin` guard is the security boundary.
- **Gap**: `handlers/agent_skill.go:23` returns hard-coded `available: false`, and `admin.go:128,267` default `disabled_filesystem_skills=true`. Frontend never offers the toggle.

### 3.2 Create Files Agent
- **Node**: 5 sub-skills (pptx, txt, pdf, xlsx, docx). Same `isToolAvailable()` gate. Files saved to `<storage>/generated-files`. Emits `fileDownloadCard` socket event + `registerOutput`.
- **Go (current)**: `agent/tools/create_files_agent.go` (153 LoC) supports txt/md/docx/pdf/xlsx via a `format` discriminator. **pptx returns a clean error** ("no permissive-licensed library"). Already registered in `builder.go:94`. Output dir: `cfg.AgentCreateFilesDir`.
- **Gap**: Same `is-available` stub. Also missing pptx.

### 3.3 Prompt History
- **Node**: `prompt_history` table (workspaceId, prompt, modifiedBy*, modifiedAt). `PromptHistory.handlePromptChange(prevData, user)` is called from `_trackWorkspacePromptChange` before persisting workspace updates, **but only** when:
  1. new openAiPrompt is non-null
  2. previous openAiPrompt is non-null
  3. previous openAiPrompt ≠ defaultPrompt
  4. previous ≠ new
- 3 endpoints: `GET /workspace/:slug/prompt-history`, `DELETE /workspace/:slug/prompt-history`, `DELETE /workspace/prompt-history/:id`. No dedicated restore endpoint — frontend reads + PUT.
- **Go**: Nothing — no model, no hook, no routes.

## 4. Design

### 4.1 PR1 · Wire FS + CreateFiles availability (~80 LoC)

**Files**:
- `backend/internal/handlers/agent_skill.go` — replace hard-coded `false` with config-backed reads:
  ```go
  func (h *AgentSkillHandler) FileSystemAgentAvailable(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"available": h.cfg.AgentFilesystemEnabled})
  }
  func (h *AgentSkillHandler) CreateFilesAgentAvailable(c *gin.Context) {
      c.JSON(http.StatusOK, gin.H{"available": h.cfg.AgentCreateFilesEnabled})
  }
  ```
  Inject `cfg` via the handler constructor.
- `backend/internal/handlers/admin.go:128, 267` — default `disabled_filesystem_skills` and `disabled_create_files_skills` from `true` → `false`.
- `backend/internal/handlers/admin.go:201` — schema/test update if needed.

**Tests**:
- `agent_skill_test.go` — exercise both endpoints with `AgentFilesystemEnabled=true/false` and assert response body.
- `admin_test.go` — assert defaults match new values.

**Migration**: none — `cfg.AgentFilesystemEnabled` / `cfg.AgentCreateFilesEnabled` already default to `true` in `config.go:289,291`.

### 4.2 PR2 · pptx via unioffice (~300 LoC) — **license-gated**

**License risk**: `github.com/unidoc/unioffice` is dual-licensed AGPL-3.0 / commercial. AGPL imposes source-disclosure obligations on network-served deployments. **PR2 cannot merge until license question is resolved** by the project owner; PR1 and PR3 are independent and can ship first.

**Files**:
- `backend/go.mod` — add `github.com/unidoc/unioffice` at the version current at PR time.
- `backend/internal/agent/tools/create_files_pptx.go` (new) — `writePptxFile(ctx context.Context, dst, content, filename string) error`. Input convention (matching anything-llm's "simple slide content"):
  - Slides separated by a line containing exactly `---`.
  - First non-empty line of each slide → title.
  - Remaining lines → body bullets.
- `backend/internal/agent/tools/create_files_agent.go:46` — replace the early `pptx` return with `writePptxFile(...)` call mirroring `docx` path.

**Tests** (`create_files_pptx_test.go`):
- Single-slide ASCII content → file opens in PowerPoint (manual + checksum-stable byte count assertion).
- Three slides separated by `---` → 3 slides assertion via reading pptx zip.
- Empty content → 1 empty slide (no crash).

### 4.3 PR3 · Prompt History (~250 LoC)

**Files**:
- `backend/internal/models/prompt_history.go` (new):
  ```go
  type PromptHistory struct {
      ID          int    `gorm:"primaryKey;autoIncrement" json:"id"`
      WorkspaceID int    `gorm:"index;not null" json:"workspaceId"`
      Prompt      string `gorm:"type:text;not null" json:"prompt"`
      ModifiedBy  *int   `gorm:"index" json:"modifiedBy,omitempty"`
      ModifiedAt  time.Time `gorm:"autoCreateTime" json:"modifiedAt"`
  }
  ```
  Add to `services.AutoMigrate` registration list.

- `backend/internal/services/prompt_history.go` (new):
  - `Log(ctx, workspaceID int, prevPrompt string, userID *int) error`
  - `ListByWorkspace(ctx, workspaceID int, limit int) ([]PromptHistory, error)` — default `limit=50`, joined `users.username` projection
  - `Delete(ctx, id int) error`
  - `DeleteAll(ctx, workspaceID int) error`

- `backend/internal/services/workspace.go` (existing — modify `Update`):
  In `Update(ctx, slug, fields)`, after loading `prev`, **before** writing `prev.OpenAiPrompt = newPrompt`, replicate anything-llm's 4-condition filter:
  ```go
  if newPrompt != nil && *newPrompt != "" &&
     prev.OpenAiPrompt != "" &&
     prev.OpenAiPrompt != DefaultPrompt &&
     prev.OpenAiPrompt != *newPrompt {
      _ = h.promptHistorySvc.Log(ctx, prev.ID, prev.OpenAiPrompt, userIDFromCtx(ctx))
  }
  ```
  Log failure is non-fatal (warn only).

- `backend/internal/handlers/workspace.go` (existing — add 3 routes):
  - `GET    /workspace/:slug/prompt-history`     → `ListByWorkspace`
  - `DELETE /workspace/:slug/prompt-history`     → `DeleteAll`
  - `DELETE /workspace/prompt-history/:id`       → `Delete`

  Auth: `ValidatedRequest` + workspace-membership middleware (mirror existing workspace route guards).

**No new index** beyond `(workspaceId)` and `(modifiedBy)` — 50-row/workspace volume.

**Restore**: no dedicated endpoint. Frontend reads history, calls existing `PATCH /workspace/:slug` with the chosen `openAiPrompt`. This naturally creates a new history entry (the prior prompt becomes the new "previous").

**Tests**:
- `prompt_history_test.go` — Log filter (all 4 anything-llm conditions exercised individually), list ordering, delete-one, delete-all
- Integration test: create workspace → update prompt 3× → assert history has 2 rows in reverse-chronological order

## 5. Risk register

| # | Risk | Mitigation |
|---|------|------------|
| R1 | unioffice license (AGPL-3.0 / commercial) | PR2 marked "blocked on legal/owner sign-off"; PR1+PR3 ship independently. Risk surfaced at design time |
| R2 | `disabled_*_skills` default flip causes silent new-skill availability on upgrade | CHANGELOG entry: "set `AGENT_FILESYSTEM_ENABLED=false` and/or `AGENT_CREATE_FILES_ENABLED=false` to preserve prior behavior" |
| R3 | Prompt-history hook in `WorkspaceService.Update` adds DB write to a hot path | Hook is fire-and-forget warn-only; only fires when all 4 conditions match (rare path — workspace prompt change) |
| R4 | `unioffice` pulls in a large transitive dependency tree | Verify with `go mod why` before merge; document binary size delta in PR2 |

## 6. Done criteria

- `go test ./backend/...` green
- `go build ./...` green
- Manual smoke:
  - `curl /agent-skills/filesystem-agent/is-available` returns `{"available":true}` by default
  - `curl /agent-skills/create-files-agent/is-available` returns `{"available":true}` by default
  - Agent invoking `create-files-agent` with `format=pptx` produces a file openable in PowerPoint (PR2 only)
  - Update workspace `openAiPrompt` twice → `GET /workspace/:slug/prompt-history` returns 1 row pointing to the original prompt
- CHANGELOG entry for default-flip + pptx capability (if PR2 lands)

## 7. PR sequencing

1. **PR1** (FS + CreateFiles wiring) — independent, can ship Day 1
2. **PR3** (Prompt History) — independent, can ship in parallel with PR1
3. **PR2** (unioffice pptx) — gated on license sign-off; can land anytime after that, no dependency on PR1/PR3

## 8. Open questions

None at design time. All decisions resolved during brainstorm:
- pptx implementation: unioffice (license-gated)
- admin default `disabled_*_skills`: false (default-enable)
- Prompt history field scope: only `OpenAiPrompt`, exact anything-llm parity
- Prompt history pagination: none (50-row default cap)
- Prompt history restore endpoint: none (reuse PATCH `/workspace/:slug`)
