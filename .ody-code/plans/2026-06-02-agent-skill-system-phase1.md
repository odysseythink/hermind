# Agent Skill System 增强 — Phase 1 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 扩展 AgentSkill 数据模型，实现平台隔离、条件可见性过滤和增强型系统提示组装，使技能系统从「静态存储」升级为「动态、平台感知、条件激活」的能力单元。

**Architecture:** 保留 Hermind 现有 DB 模型和 REST API 骨架，新增 6 个 JSON text 列用于结构化查询，在 WebSocket 会话启动路径中插入平台/条件过滤层，替换现有系统提示为强制加载语义 + 分类索引格式。

**Tech Stack:** Go 1.26, Gin, GORM (SQLite/PostgreSQL), Pantheon SDK

**Depends on file:** `2026-06-02-agent-skill-system-index.md`

---

## File Structure

### 修改文件

| 文件 | 责任 |
|------|------|
| `backend/internal/models/agent_skill.go` | 扩展 AgentSkill 结构体：新增 6 个 JSON text 列 + UsageSidecar + Parse* 辅助方法 |
| `backend/internal/services/agent_skill_service.go` | 扩展 ListActiveByWorkspace 签名（加入平台过滤 + 条件过滤）；Create/Update 提取 frontmatter 字段到新列 |
| `backend/internal/agent/system_prompt.go` | 重写 resolveSystemPrompt：增强型强制加载语义 + 分类索引 |
| `backend/internal/agent/handler.go` | 重组会话启动顺序：预览注册表 → 提取可用工具 → 过滤技能 → 组装系统提示 |
| `backend/internal/agent/runtime.go` | 同上重组（runtime 路径复用相同逻辑） |

### 新增文件

| 文件 | 责任 |
|------|------|
| `backend/internal/services/skill_frontmatter.go` | YAML frontmatter 字段提取器 |
| `backend/internal/services/skill_platform.go` | runtime.GOOS 平台匹配过滤 |
| `backend/internal/services/skill_filter.go` | 条件可见性过滤（requires_tools / requires_toolsets 子集检查） |
| `backend/internal/services/skill_platform_test.go` | 平台过滤单元测试 |
| `backend/internal/services/skill_filter_test.go` | 条件过滤单元测试 |
| `backend/internal/agent/system_prompt_test.go` | 系统提示组装单元测试 |

---

## Dependency Overview

```
Task 1 (model + interface + shared signature)
  ├──► Task 2 (frontmatter parser — 依赖模型新字段)
  ├──► Task 3 (platform filtering — 依赖 ParsePlatforms)
  ├──► Task 4 (conditional filtering — 依赖 ParseRequires*)
  │
  Task 5 (system prompt — 依赖过滤后的技能列表)
  │
  Task 6 (handler integration — 依赖 Task 1-5 全部)
  │
  Task 7 (integration tests — 依赖 Task 1-6)
```

**可并行：** Task 2、3、4 在 Task 1 完成后可并行开发；Task 5 在 Task 3/4 完成后开发。

---

## Risks & Open Questions

| 风险 | 缓解 |
|------|------|
| GORM AutoMigrate 对新增 JSON text 列在 PostgreSQL 下默认值表现不同 | Task 1 包含 SQLite + PostgreSQL 双 DB 迁移验证 |
| 增强型系统提示 token 增加 | Task 4 条件过滤已减少注入技能数量；Task 6 保留零技能时的占位提示 |
| `runtime.GOOS` 值不足以覆盖部署场景 | 平台映射表可扩展；Task 3 测试覆盖 macos→darwin 映射 |

---

## Tasks

### Task 1: 扩展 AgentSkill 模型 + Parse* 方法 + ListActiveByWorkspace 签名变更

**Depends on:** none

**Files:**
- Modify: `backend/internal/models/agent_skill.go`
- Modify: `backend/internal/services/agent_skill_service.go`（接口定义 + ListActiveByWorkspace 实现）
- Modify: `backend/internal/agent/handler.go`（caller 更新）
- Modify: `backend/internal/agent/runtime.go`（caller 更新）
- Modify: `backend/internal/services/agent_skill_service_test.go`（测试 caller 更新）
- Modify: `backend/internal/agent/runtime_test.go`（如有，caller 更新）

> **这是本 Phase 唯一的共享签名变更任务。** 所有模型字段、接口方法、调用点在此任务一次性完成，避免跨任务重复更新调用者。

- [ ] **Step 1: 修改 AgentSkill 模型，新增 6 个 JSON text 列 + UsageSidecar**

在 `backend/internal/models/agent_skill.go` 的 `AgentSkill` 结构体中，在 `Description` 字段之前插入以下字段：

```go
// 从 frontmatter 中提取为独立列，用于查询过滤
Platforms           string `gorm:"type:text;default:''" json:"platforms"`
RequiresTools       string `gorm:"type:text;default:''" json:"requiresTools"`
RequiresToolsets    string `gorm:"type:text;default:''" json:"requiresToolsets"`
FallbackForTools    string `gorm:"type:text;default:''" json:"fallbackForTools"`
FallbackForToolsets string `gorm:"type:text;default:''" json:"fallbackForToolsets"`
ConfigVars          string `gorm:"type:text;default:''" json:"configVars"`

// 侧车 JSON 列（替代 .usage.json 文件）
UsageSidecar        string `gorm:"type:text;default:'{}'" json:"usageSidecar"`
```

- [ ] **Step 2: 添加 Parse* 辅助方法到 AgentSkill**

在 `backend/internal/models/agent_skill.go` 末尾添加：

```go
func (s *AgentSkill) parseJSONStringSlice(field string) []string {
    if field == "" || field == "null" {
        return nil
    }
    var arr []string
    if err := json.Unmarshal([]byte(field), &arr); err != nil {
        return nil
    }
    return arr
}

func (s *AgentSkill) ParsePlatforms() []string        { return s.parseJSONStringSlice(s.Platforms) }
func (s *AgentSkill) ParseRequiresTools() []string    { return s.parseJSONStringSlice(s.RequiresTools) }
func (s *AgentSkill) ParseRequiresToolsets() []string { return s.parseJSONStringSlice(s.RequiresToolsets) }
func (s *AgentSkill) ParseFallbackForTools() []string { return s.parseJSONStringSlice(s.FallbackForTools) }
func (s *AgentSkill) ParseFallbackForToolsets() []string { return s.parseJSONStringSlice(s.FallbackForToolsets) }
```

> 注意：需要 `import "encoding/json"`。

- [ ] **Step 3: 变更 ListActiveByWorkspace 签名，实现平台过滤 + 条件过滤**

在 `backend/internal/services/agent_skill_service.go` 中：

**a) 修改接口定义（约第 64 行）：**

```go
ListActiveByWorkspace(ctx context.Context, workspaceID int, availableTools []string, availableToolsets []string) ([]models.AgentSkill, error)
```

**b) 修改实现（约第 276 行）：**

```go
func (s *AgentSkillService) ListActiveByWorkspace(ctx context.Context, workspaceID int, availableTools []string, availableToolsets []string) ([]models.AgentSkill, error) {
    var skills []models.AgentSkill
    if err := s.db.WithContext(ctx).
        Where("workspace_id = ? AND status = ?", workspaceID, models.AgentSkillStatusActive).
        Order("category ASC, name ASC").
        Find(&skills).Error; err != nil {
        return nil, err
    }
    // 平台过滤 + 条件过滤
    filtered := make([]models.AgentSkill, 0, len(skills))
    for _, skill := range skills {
        if !skillMatchesPlatform(&skill) {
            continue
        }
        if !skillMatchesConditions(&skill, availableTools, availableToolsets) {
            continue
        }
        filtered = append(filtered, skill)
    }
    return filtered, nil
}
```

> `skillMatchesPlatform` 和 `skillMatchesConditions` 是将在 Task 3 和 Task 4 中定义的函数。由于 Go 允许调用同一 package 中的未定义函数（只要它们在编译前存在），而 Task 3/4 会在此任务之后完成，所以这里先引用。如果执行顺序要求编译通过，可以先用临时占位函数。

**c) 在同一文件中，于 `patchContent` 之后添加临时占位函数（Task 3/4 完成后会被替换）：**

```go
// TODO: replaced by Task 3 / Task 4
func skillMatchesPlatform(skill *models.AgentSkill) bool       { return true }
func skillMatchesConditions(skill *models.AgentSkill, tools, toolsets []string) bool { return true }
```

- [ ] **Step 4: 查找并更新所有 ListActiveByWorkspace 调用者**

Run: `grep -rn "ListActiveByWorkspace" backend/`

更新以下调用点，传入 `nil, nil` 作为可用工具参数（表示不过滤）：

**a) `backend/internal/agent/handler.go` 约第 56-58 行：**

```go
var skills []models.AgentSkill
if r.deps.AgentSkillSvc != nil {
    skills, _ = r.deps.AgentSkillSvc.ListActiveByWorkspace(c.Request.Context(), ws.ID, nil, nil)
}
```

**b) `backend/internal/agent/runtime.go` 约第 165-166 行：**

```go
var skills []models.AgentSkill
if r.deps.AgentSkillSvc != nil {
    skills, _ = r.deps.AgentSkillSvc.ListActiveByWorkspace(ctx, ws.ID, nil, nil)
}
```

**c) `backend/internal/services/agent_skill_service_test.go`：**

搜索 `ListActiveByWorkspace` 的所有测试调用，将 `svc.ListActiveByWorkspace(ctx, 1)` 改为 `svc.ListActiveByWorkspace(ctx, 1, nil, nil)`。

- [ ] **Step 5: 更新 Create/Update 以持久化 frontmatter 提取字段**

在 `backend/internal/services/agent_skill_service.go` 的 `Create` 方法中，创建 `models.AgentSkill` 之前，从已解析的 frontmatter 中提取字段：

```go
// 在 skill := models.AgentSkill{...} 之前添加提取逻辑
platforms, requiresTools, requiresToolsets, fallbackForTools, fallbackForToolsets, configVars :=
    extractFrontmatterFields(frontmatter)

skill := models.AgentSkill{
    // ... 现有字段 ...
    Platforms:           platforms,
    RequiresTools:       requiresTools,
    RequiresToolsets:    requiresToolsets,
    FallbackForTools:    fallbackForTools,
    FallbackForToolsets: fallbackForToolsets,
    ConfigVars:          configVars,
    // ...
}
```

在 `Update` 方法中，如果 `req.Frontmatter != ""`，同样提取并加入 `updates` map。

> `extractFrontmatterFields` 是 Task 2 中定义的函数。在此任务中先添加临时占位：

```go
func extractFrontmatterFields(frontmatter string) (platforms, requiresTools, requiresToolsets, fallbackForTools, fallbackForToolsets, configVars string) {
    return "", "", "", "", "", ""
}
```

- [ ] **Step 6: 全树类型检查（包含测试文件）**

Run:
```bash
cd backend && go vet ./...
```
Expected: 通过，无类型错误。

> 注意：使用 `go vet ./...` 而非 `go build ./...`，因为后者不编译 `_test.go` 文件，会漏掉测试中的 stale caller。

- [ ] **Step 7: 运行现有测试，确保无回归**

Run:
```bash
cd backend && go test ./internal/models/... ./internal/services/... -v -count=1
```
Expected: 所有现有测试通过。

- [ ] **Step 8: 提交**

```bash
git add backend/internal/models/agent_skill.go backend/internal/services/agent_skill_service.go backend/internal/agent/handler.go backend/internal/agent/runtime.go backend/internal/services/agent_skill_service_test.go
git commit -m "feat(agent-skill): extend AgentSkill model with 6 JSON columns + UsageSidecar, update ListActiveByWorkspace signature"
```

---

### Task 2: Frontmatter 解析器与字段提取

**Depends on:** Task 1

**Files:**
- Create: `backend/internal/services/skill_frontmatter.go`
- Create: `backend/internal/services/skill_frontmatter_test.go`
- Modify: `backend/internal/services/agent_skill_service.go`（替换占位函数）

- [ ] **Step 1: 编写失败的单元测试**

创建 `backend/internal/services/skill_frontmatter_test.go`：

```go
package services

import (
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestExtractFrontmatterFields(t *testing.T) {
    fm := `name: deploy-k8s
description: Deploy to Kubernetes
platforms:
  - linux
  - macos
requires_tools:
  - kubectl
  - helm
requires_toolsets:
  - terminal
fallback_for_tools:
  - docker
custom_field: ignored
`
    platforms, reqTools, reqToolsets, fbTools, fbToolsets, configVars := extractFrontmatterFields(fm)

    assert.Equal(t, `["linux","macos"]`, platforms)
    assert.Equal(t, `["kubectl","helm"]`, reqTools)
    assert.Equal(t, `["terminal"]`, reqToolsets)
    assert.Equal(t, `["docker"]`, fbTools)
    assert.Equal(t, ``, fbToolsets)
    assert.Equal(t, ``, configVars)
}

func TestExtractFrontmatterFields_Empty(t *testing.T) {
    platforms, reqTools, reqToolsets, fbTools, fbToolsets, configVars := extractFrontmatterFields("")
    assert.Equal(t, "", platforms)
    assert.Equal(t, "", reqTools)
    assert.Equal(t, "", reqToolsets)
    assert.Equal(t, "", fbTools)
    assert.Equal(t, "", fbToolsets)
    assert.Equal(t, "", configVars)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd backend && go test ./internal/services/ -run TestExtractFrontmatterFields -v
```
Expected: FAIL with "function not defined"

- [ ] **Step 3: 实现 frontmatter 字段提取器**

创建 `backend/internal/services/skill_frontmatter.go`：

```go
package services

import (
    "encoding/json"

    "gopkg.in/yaml.v3"
)

// extractFrontmatterFields 从 YAML frontmatter 中提取结构化字段，
// 返回 JSON 序列化后的字符串（用于直接存入 GORM text 列）。
func extractFrontmatterFields(frontmatter string) (platforms, requiresTools, requiresToolsets, fallbackForTools, fallbackForToolsets, configVars string) {
    if frontmatter == "" {
        return "", "", "", "", "", ""
    }

    var fm map[string]any
    if err := yaml.Unmarshal([]byte(frontmatter), &fm); err != nil {
        return "", "", "", "", "", ""
    }

    platforms = marshalStringSlice(fm, "platforms")
    requiresTools = marshalStringSlice(fm, "requires_tools")
    requiresToolsets = marshalStringSlice(fm, "requires_toolsets")
    fallbackForTools = marshalStringSlice(fm, "fallback_for_tools")
    fallbackForToolsets = marshalStringSlice(fm, "fallback_for_toolsets")
    configVars = marshalConfigVars(fm)

    return platforms, requiresTools, requiresToolsets, fallbackForTools, fallbackForToolsets, configVars
}

func marshalStringSlice(fm map[string]any, key string) string {
    raw, ok := fm[key]
    if !ok {
        return ""
    }
    var arr []string
    switch v := raw.(type) {
    case []any:
        for _, item := range v {
            if s, ok := item.(string); ok {
                arr = append(arr, s)
            }
        }
    case []string:
        arr = v
    case string:
        if v != "" {
            arr = []string{v}
        }
    }
    if len(arr) == 0 {
        return ""
    }
    b, _ := json.Marshal(arr)
    return string(b)
}

func marshalConfigVars(fm map[string]any) string {
    raw, ok := fm["config_vars"]
    if !ok {
        return ""
    }
    b, _ := json.Marshal(raw)
    return string(b)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
cd backend && go test ./internal/services/ -run TestExtractFrontmatterFields -v
```
Expected: PASS

- [ ] **Step 5: 替换 agent_skill_service.go 中的占位函数**

在 `backend/internal/services/agent_skill_service.go` 中，删除 Step 1 中创建的临时占位 `extractFrontmatterFields`，确认其已被新文件中的实现替代。

Run:
```bash
cd backend && go vet ./internal/services/
```
Expected: PASS（无重复定义错误）。

- [ ] **Step 6: 更新 Create/Update 以使用真实提取结果**

在 `Create` 方法中，确认 `extractFrontmatterFields(frontmatter)` 的返回值被正确赋给 `models.AgentSkill` 的新字段（已在 Task 1 Step 5 中完成，此处只需验证）。

在 `Update` 方法中，如果 `req.Frontmatter != ""`，在 `validateFrontmatter` 之后添加：

```go
if req.Frontmatter != "" {
    if _, err := validateFrontmatter(req.Frontmatter); err != nil {
        return nil, err
    }
    updates["frontmatter"] = req.Frontmatter
    platforms, reqTools, reqToolsets, fbTools, fbToolsets, cfgVars := extractFrontmatterFields(req.Frontmatter)
    updates["platforms"] = platforms
    updates["requires_tools"] = reqTools
    updates["requires_toolsets"] = reqToolsets
    updates["fallback_for_tools"] = fbTools
    updates["fallback_for_toolsets"] = fbToolsets
    updates["config_vars"] = cfgVars
}
```

- [ ] **Step 7: 全树类型检查 + 提交**

Run:
```bash
cd backend && go vet ./...
```
Expected: PASS

```bash
git add backend/internal/services/skill_frontmatter.go backend/internal/services/skill_frontmatter_test.go backend/internal/services/agent_skill_service.go
git commit -m "feat(agent-skill): add frontmatter field extractor for structured columns"
```

---

### Task 3: 平台过滤（runtime.GOOS 匹配）

**Depends on:** Task 1

**Files:**
- Create: `backend/internal/services/skill_platform.go`
- Create: `backend/internal/services/skill_platform_test.go`
- Modify: `backend/internal/services/agent_skill_service.go`（替换占位函数）

- [ ] **Step 1: 编写失败的单元测试**

创建 `backend/internal/services/skill_platform_test.go`：

```go
package services

import (
    "runtime"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
)

func TestSkillMatchesPlatform(t *testing.T) {
    goos := runtime.GOOS

    // 无 platforms 字段 = 全平台匹配
    assert.True(t, skillMatchesPlatform(&models.AgentSkill{Platforms: ""}))
    assert.True(t, skillMatchesPlatform(&models.AgentSkill{Platforms: "[]"}))

    // 当前平台匹配
    assert.True(t, skillMatchesPlatform(&models.AgentSkill{Platforms: `["` + goos + `"]`}))

    // macos → darwin 映射
    if goos == "darwin" {
        assert.True(t, skillMatchesPlatform(&models.AgentSkill{Platforms: `["macos"]`}))
    }

    // 不匹配
    other := "windows"
    if goos == "windows" {
        other = "linux"
    }
    assert.False(t, skillMatchesPlatform(&models.AgentSkill{Platforms: `["` + other + `"]`}))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd backend && go test ./internal/services/ -run TestSkillMatchesPlatform -v
```
Expected: FAIL with "function not defined"

- [ ] **Step 3: 实现平台过滤逻辑**

创建 `backend/internal/services/skill_platform.go`：

```go
package services

import (
    "runtime"
    "strings"

    "github.com/odysseythink/hermind/backend/internal/models"
)

var platformMap = map[string]string{
    "macos":   "darwin",
    "linux":   "linux",
    "windows": "windows",
}

// skillMatchesPlatform 检查技能是否匹配当前运行平台。
// 无 platforms 字段时返回 true（全平台可用）。
func skillMatchesPlatform(skill *models.AgentSkill) bool {
    platforms := skill.ParsePlatforms()
    if len(platforms) == 0 {
        return true
    }
    goos := runtime.GOOS
    for _, p := range platforms {
        normalized := strings.ToLower(strings.TrimSpace(p))
        mapped, ok := platformMap[normalized]
        if !ok {
            mapped = normalized
        }
        if goos == mapped {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
cd backend && go test ./internal/services/ -run TestSkillMatchesPlatform -v
```
Expected: PASS

- [ ] **Step 5: 替换 agent_skill_service.go 中的占位函数**

在 `backend/internal/services/agent_skill_service.go` 中，删除 Task 1 中添加的临时占位 `skillMatchesPlatform`。

Run:
```bash
cd backend && go vet ./...
```
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/services/skill_platform.go backend/internal/services/skill_platform_test.go backend/internal/services/agent_skill_service.go
git commit -m "feat(agent-skill): add platform filtering with GOOS matching"
```

---

### Task 4: 条件可见性过滤（requires_tools / requires_toolsets）

**Depends on:** Task 1

**Files:**
- Create: `backend/internal/services/skill_filter.go`
- Create: `backend/internal/services/skill_filter_test.go`
- Modify: `backend/internal/services/agent_skill_service.go`（替换占位函数）

- [ ] **Step 1: 编写失败的单元测试**

创建 `backend/internal/services/skill_filter_test.go`：

```go
package services

import (
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
)

func TestSkillMatchesConditions(t *testing.T) {
    availableTools := map[string]struct{}{"kubectl": {}, "helm": {}}
    availableToolsets := map[string]struct{}{"terminal": {}}

    // 完全匹配
    skill := &models.AgentSkill{
        RequiresTools:    `["kubectl"]`,
        RequiresToolsets: `["terminal"]`,
    }
    assert.True(t, skillMatchesConditions(skill, availableTools, availableToolsets))

    // 无需条件 = 匹配
    skill2 := &models.AgentSkill{}
    assert.True(t, skillMatchesConditions(skill2, availableTools, availableToolsets))

    // 缺少工具
    skill3 := &models.AgentSkill{RequiresTools: `["docker"]`}
    assert.False(t, skillMatchesConditions(skill3, availableTools, availableToolsets))

    // 缺少 toolset
    skill4 := &models.AgentSkill{RequiresToolsets: `["web"]`}
    assert.False(t, skillMatchesConditions(skill4, availableTools, availableToolsets))

    // 部分工具匹配但非全部
    skill5 := &models.AgentSkill{RequiresTools: `["kubectl","docker"]`}
    assert.False(t, skillMatchesConditions(skill5, availableTools, availableToolsets))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd backend && go test ./internal/services/ -run TestSkillMatchesConditions -v
```
Expected: FAIL with "function not defined"

- [ ] **Step 3: 实现条件过滤逻辑**

创建 `backend/internal/services/skill_filter.go`：

```go
package services

import (
    "github.com/odysseythink/hermind/backend/internal/models"
)

// skillMatchesConditions 检查技能的 requires_tools / requires_toolsets
// 是否是当前可用工具集的子集。空条件视为无条件匹配。
func skillMatchesConditions(skill *models.AgentSkill, availableTools map[string]struct{}, availableToolsets map[string]struct{}) bool {
    reqTools := skill.ParseRequiresTools()
    if !isSubset(reqTools, availableTools) {
        return false
    }
    reqToolsets := skill.ParseRequiresToolsets()
    if !isSubset(reqToolsets, availableToolsets) {
        return false
    }
    return true
}

func isSubset(items []string, set map[string]struct{}) bool {
    if len(items) == 0 {
        return true
    }
    if len(set) == 0 {
        return false
    }
    for _, item := range items {
        if _, ok := set[item]; !ok {
            return false
        }
    }
    return true
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
cd backend && go test ./internal/services/ -run TestSkillMatchesConditions -v
```
Expected: PASS

- [ ] **Step 5: 替换 agent_skill_service.go 中的占位函数**

在 `backend/internal/services/agent_skill_service.go` 中，删除 Task 1 中添加的临时占位 `skillMatchesConditions`。

Run:
```bash
cd backend && go vet ./...
```
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add backend/internal/services/skill_filter.go backend/internal/services/skill_filter_test.go backend/internal/services/agent_skill_service.go
git commit -m "feat(agent-skill): add conditional visibility filtering by tools/toolsets"
```

---

### Task 5: 增强型系统提示组装

**Depends on:** Task 3, Task 4

**Files:**
- Modify: `backend/internal/agent/system_prompt.go`
- Create: `backend/internal/agent/system_prompt_test.go`

- [ ] **Step 1: 编写失败的单元测试**

创建 `backend/internal/agent/system_prompt_test.go`：

```go
package agent

import (
    "strings"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/stretchr/testify/assert"
)

func TestBuildEnhancedSkillsPrompt(t *testing.T) {
    skills := []models.AgentSkill{
        {Slug: "deploy-k8s", Description: "Deploy to Kubernetes", Category: "devops"},
        {Slug: "lint-go", Description: "Lint Go code", Category: "devops"},
        {Slug: "write-tests", Description: "Write unit tests", Category: "qa"},
    }

    prompt := buildEnhancedSkillsPrompt(skills)

    assert.Contains(t, prompt, "## Skills (mandatory)")
    assert.Contains(t, prompt, "skill_view(name)")
    assert.Contains(t, prompt, "devops:")
    assert.Contains(t, prompt, "- deploy-k8s: Deploy to Kubernetes")
    assert.Contains(t, prompt, "qa:")
    assert.Contains(t, prompt, "<available_skills>")
    assert.Contains(t, prompt, "</available_skills>")
}

func TestBuildEnhancedSkillsPrompt_Empty(t *testing.T) {
    prompt := buildEnhancedSkillsPrompt(nil)
    assert.Equal(t, "", prompt)
}

func TestResolveSystemPrompt_WithSkills(t *testing.T) {
    ws := &models.Workspace{OpenAiPrompt: strPtr("You are a helpful assistant.")}
    skills := []models.AgentSkill{
        {Slug: "deploy-k8s", Description: "Deploy", Category: "devops"},
    }

    prompt := resolveSystemPrompt(ws, nil, skills)

    assert.True(t, strings.HasPrefix(prompt, "You are a helpful assistant."))
    assert.Contains(t, prompt, "## Skills (mandatory)")
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: 运行测试确认失败**

Run:
```bash
cd backend && go test ./internal/agent/ -run TestBuildEnhancedSkillsPrompt -v
```
Expected: FAIL with "function not defined"

- [ ] **Step 3: 实现增强型系统提示组装**

重写 `backend/internal/agent/system_prompt.go`：

```go
package agent

import (
    "sort"
    "strings"

    "github.com/odysseythink/hermind/backend/internal/models"
)

const defaultSystemPrompt = `You are a helpful AI assistant. You can use available tools to answer the user's questions.`

func resolveSystemPrompt(ws *models.Workspace, user *models.User, skills []models.AgentSkill) string {
    var sb strings.Builder
    if ws != nil && ws.OpenAiPrompt != nil && *ws.OpenAiPrompt != "" {
        sb.WriteString(*ws.OpenAiPrompt)
    } else {
        sb.WriteString(defaultSystemPrompt)
    }

    if len(skills) > 0 {
        sb.WriteString("\n\n")
        sb.WriteString(buildEnhancedSkillsPrompt(skills))
    }

    return sb.String()
}

func buildEnhancedSkillsPrompt(skills []models.AgentSkill) string {
    if len(skills) == 0 {
        return ""
    }

    var b strings.Builder
    b.WriteString("## Skills (mandatory)\n")
    b.WriteString("Before replying, scan the skills below. If a skill matches or is even partially relevant ")
    b.WriteString("to your task, you MUST load it with skill_view(name) and follow its instructions. ")
    b.WriteString("Err on the side of loading — it is always better to have context you don't need ")
    b.WriteString("than to miss critical steps, pitfalls, or established workflows.\n\n")

    b.WriteString("<available_skills>\n")
    byCategory := groupSkillsByCategory(skills)
    for _, cat := range sortedKeys(byCategory) {
        b.WriteString("  ")
        b.WriteString(cat)
        b.WriteString(":\n")
        for _, skill := range byCategory[cat] {
            b.WriteString("    - ")
            b.WriteString(skill.Slug)
            b.WriteString(": ")
            if skill.Description != "" {
                b.WriteString(skill.Description)
            }
            b.WriteString("\n")
        }
    }
    b.WriteString("</available_skills>\n\n")
    b.WriteString("Only proceed without loading a skill if genuinely none are relevant to the task.")
    return b.String()
}

func groupSkillsByCategory(skills []models.AgentSkill) map[string][]models.AgentSkill {
    groups := make(map[string][]models.AgentSkill)
    for _, s := range skills {
        cat := s.Category
        if cat == "" {
            cat = "general"
        }
        groups[cat] = append(groups[cat], s)
    }
    return groups
}

func sortedKeys(m map[string][]models.AgentSkill) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    return keys
}

// ResolveSystemPromptForTesting exposes resolveSystemPrompt for unit tests.
func ResolveSystemPromptForTesting(ws *models.Workspace, user *models.User) string {
    return resolveSystemPrompt(ws, user, nil)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:
```bash
cd backend && go test ./internal/agent/ -run TestBuildEnhancedSkillsPrompt -v
```
Expected: PASS

- [ ] **Step 5: 构建检查 + 提交**

Run:
```bash
cd backend && go vet ./...
```
Expected: PASS

```bash
git add backend/internal/agent/system_prompt.go backend/internal/agent/system_prompt_test.go
git commit -m "feat(agent-skill): rewrite system prompt with mandatory loading semantics and categorized skill index"
```

---

### Task 6: Handler 集成 — 可用工具提取与会话启动重排

**Depends on:** Task 1, Task 2, Task 3, Task 4, Task 5

**Files:**
- Modify: `backend/internal/agent/handler.go`
- Modify: `backend/internal/agent/runtime.go`
- Modify: `backend/internal/services/agent_skill_service.go`（如有需要调整 ListActiveByWorkspace 调用）

> **核心变更：** 解决「鸡生蛋蛋生鸡」问题——条件过滤需要可用工具列表，但工具注册表在当前代码流中是在系统提示之后构建的。解决方式是先构建预览注册表（无 approval）提取工具名，再过滤技能，最后构建真实注册表。

- [ ] **Step 1: 添加可用工具提取辅助函数**

在 `backend/internal/agent/handler.go` 中添加（放在 `buildSessionRegistry` 函数附近）：

```go
// extractAvailableToolInfo 从注册表中提取所有可用工具名和工具集名。
func extractAvailableToolInfo(reg *tool.Registry) (tools []string, toolsets []string) {
    seenToolsets := make(map[string]struct{})
    for _, e := range reg.Entries(nil) {
        tools = append(tools, e.Name)
        if e.Toolset != "" {
            seenToolsets[e.Toolset] = struct{}{}
        }
    }
    for ts := range seenToolsets {
        toolsets = append(toolsets, ts)
    }
    return tools, toolsets
}
```

> 需要确认 `tool.Registry` 有 `Entries` 方法。在 Pantheon v0.0.8 中已确认存在。

- [ ] **Step 2: 重组 handler.go 的 HandleWS 方法**

将 `backend/internal/agent/handler.go` 中约第 55-101 行的逻辑从：

```
fetch skills → build system prompt → create session → build registry → initAgent
```

重排为：

```
build preview registry → extract tools → fetch filtered skills → build system prompt → create session → build real registry → initAgent
```

具体修改（约第 55-101 行）：

```go
// 步骤 1: 构建预览注册表以发现可用工具（approval=nil，避免触发 approval gate）
var availableTools, availableToolsets []string
previewReg, err := buildSessionRegistry(c.Request.Context(), r.deps, &ws, user, lm, settings, nil, nil)
if err != nil {
    _ = wc.Send(ServerFrame{Type: FrameWSSFailure, Content: "tools: " + err.Error()})
    return
}
availableTools, availableToolsets = extractAvailableToolInfo(previewReg)

// 步骤 2: 使用可用工具信息获取过滤后的技能
var skills []models.AgentSkill
if r.deps.AgentSkillSvc != nil {
    skills, _ = r.deps.AgentSkillSvc.ListActiveByWorkspace(c.Request.Context(), ws.ID, availableTools, availableToolsets)
}

// 步骤 3: 组装增强型系统提示
systemPrompt := resolveSystemPrompt(&ws, user, skills)

// 步骤 4-7: 创建 session、构建真实注册表、initAgent（与现有逻辑相同）
ttl := r.deps.Cfg.AgentToolApprovalTimeout
if ttl <= 0 {
    ttl = 2 * time.Minute
}
sessTTL := r.deps.Cfg.AgentSessionMaxDuration
if sessTTL <= 0 {
    sessTTL = 30 * time.Minute
}
sessCtx, sessCancel := context.WithTimeout(c.Request.Context(), sessTTL)
defer sessCancel()

var comp agentcompression.ContextEngine
if r.testCompressorOverride != nil {
    comp = r.testCompressorOverride
} else {
    comp = buildCompressor(r.deps.DB, &ws, lm, r.deps.SysSvc, func(summary string) {
        _ = wc.Send(ServerFrame{Type: "context.compressed", Content: summary})
    })
}
sess := newSession(sessCtx, inv.UUID, &ws, user, lm, systemPrompt, tool.NewRegistry(), wc, ttl, r.deps.EventLog, comp)

reg, err := buildSessionRegistry(c.Request.Context(), r.deps, &ws, user, lm, settings, nil, sess.RequestApproval)
if err != nil {
    _ = wc.Send(ServerFrame{Type: FrameWSSFailure, Content: "tools: " + err.Error()})
    return
}
sess.initAgent(lm, reg)
```

> 注意：预览注册表和真实注册表构建了两次，这是为了获取工具名而必须付出的成本。两个注册表只有 `approval` 不同，工具集合完全相同。

- [ ] **Step 3: 对 runtime.go 应用相同的重排**

在 `backend/internal/agent/runtime.go` 中，将约第 164-189 行的逻辑同样重排：

```go
// 步骤 1: 预览注册表
previewReg, err := buildSessionRegistry(ctx, r.deps, &ws, user, lm, settings, nil, nil)
if err != nil {
    _ = io.Send(ServerFrame{Type: FrameWSSFailure, Content: "tools: " + err.Error()})
    return err
}
availableTools, availableToolsets := extractAvailableToolInfo(previewReg)

// 步骤 2: 过滤技能
var skills []models.AgentSkill
if r.deps.AgentSkillSvc != nil {
    skills, _ = r.deps.AgentSkillSvc.ListActiveByWorkspace(ctx, ws.ID, availableTools, availableToolsets)
}

// 步骤 3: 系统提示
systemPrompt := resolveSystemPrompt(&ws, user, skills)

// 步骤 4-7: session 创建、真实注册表、initAgent（与现有逻辑相同）
```

- [ ] **Step 4: 全树类型检查（包含测试文件）**

Run:
```bash
cd backend && go vet ./...
```
Expected: PASS

- [ ] **Step 5: 运行 agent 包测试**

Run:
```bash
cd backend && go test ./internal/agent/... -v -count=1
```
Expected: 所有现有测试通过。

- [ ] **Step 6: 提交**

```bash
git add backend/internal/agent/handler.go backend/internal/agent/runtime.go
git commit -m "feat(agent-skill): reorder session startup to extract available tools before skill filtering"
```

---

### Task 7: Phase 1 集成测试

**Depends on:** Task 1, Task 2, Task 3, Task 4, Task 5, Task 6

**Files:**
- Create: `backend/tests/integration/agent_skill_phase1_test.go`

- [ ] **Step 1: 编写集成测试 — WebSocket 会话启动路径**

创建 `backend/tests/integration/agent_skill_phase1_test.go`：

```go
package integration

import (
    "context"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "github.com/odysseythink/hermind/backend/internal/agent"
    "github.com/odysseythink/hermind/backend/internal/dto"
    "github.com/odysseythink/hermind/backend/internal/models"
    "github.com/odysseythink/hermind/backend/internal/services"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func TestAgentSkillPhase1_SystemPromptFiltering(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // 1. 设置测试 DB
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    require.NoError(t, db.AutoMigrate(&models.AgentSkill{}, &models.AgentSkillFile{}, &models.Workspace{}, &models.User{}))

    // 2. 创建 workspace
    ws := models.Workspace{Name: "test-ws", Slug: "test-ws", OpenAiPrompt: strPtr("You are a test assistant.")}
    require.NoError(t, db.Create(&ws).Error)

    // 3. 创建技能：一个全平台、一个仅 linux、一个需要不存在工具
    svc := services.NewAgentSkillService(db)
    ctx := context.Background()

    _, err = svc.Create(ctx, ws.ID, dto.CreateAgentSkillRequest{
        Name:        "universal-skill",
        Description: "Works everywhere",
        Category:    "general",
        Content:     "## Universal\nDo something.",
        Frontmatter: "name: universal-skill\ndescription: Works everywhere\n",
    })
    require.NoError(t, err)

    _, err = svc.Create(ctx, ws.ID, dto.CreateAgentSkillRequest{
        Name:        "linux-only",
        Description: "Linux specific",
        Category:    "infra",
        Content:     "## Linux\nDo linux thing.",
        Frontmatter: "name: linux-only\ndescription: Linux specific\nplatforms:\n  - linux\n",
    })
    require.NoError(t, err)

    _, err = svc.Create(ctx, ws.ID, dto.CreateAgentSkillRequest{
        Name:        "needs-kubectl",
        Description: "Needs kubectl tool",
        Category:    "devops",
        Content:     "## K8s\nDeploy.",
        Frontmatter: "name: needs-kubectl\ndescription: Needs kubectl tool\nrequires_tools:\n  - kubectl\n",
    })
    require.NoError(t, err)

    // 4. 验证 ListActiveByWorkspace 返回过滤结果
    // 传入 nil, nil 时不过滤（向后兼容测试）
    allSkills, err := svc.ListActiveByWorkspace(ctx, ws.ID, nil, nil)
    require.NoError(t, err)
    assert.Len(t, allSkills, 3)

    // 传入空工具集时，requires_tools 不匹配的技能被过滤
    filtered, err := svc.ListActiveByWorkspace(ctx, ws.ID, []string{}, []string{})
    require.NoError(t, err)
    // universal-skill (无条件) 和 linux-only (平台条件，取决于当前 OS) 可能通过
    // needs-kubectl (需要 kubectl 但可用工具为空) 被过滤
    var slugs []string
    for _, s := range filtered {
        slugs = append(slugs, s.Slug)
    }
    assert.NotContains(t, slugs, "needs-kubectl")

    // 传入 kubectl 工具时，needs-kubectl 出现
    withKubectl, err := svc.ListActiveByWorkspace(ctx, ws.ID, []string{"kubectl"}, []string{})
    require.NoError(t, err)
    slugs = nil
    for _, s := range withKubectl {
        slugs = append(slugs, s.Slug)
    }
    assert.Contains(t, slugs, "needs-kubectl")
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: 运行集成测试**

Run:
```bash
cd backend && go test ./tests/integration/ -run TestAgentSkillPhase1 -v
```
Expected: PASS

- [ ] **Step 3: 全量测试回归**

Run:
```bash
cd backend && go test ./... -count=1
```
Expected: 所有测试通过。

- [ ] **Step 4: 提交**

```bash
git add backend/tests/integration/agent_skill_phase1_test.go
git commit -m "test(agent-skill): add Phase 1 integration tests for platform and conditional filtering"
```

---

## Self-Review

- [ ] **1. Spec coverage (build the table).**

| Spec section | Task(s) | Status |
|---|---|---|
| §2 数据模型扩展（AgentSkill 6 个 JSON 列 + UsageSidecar） | Task 1 | covered |
| §4.1 平台过滤 | Task 3 | covered |
| §4.2 条件可见性过滤 | Task 4 | covered |
| §4.3 增强型系统提示组装 | Task 5 | covered |
| §4.4 调用点集成（handler.go / runtime.go） | Task 6 | covered |
| §2.4 SystemSetting 键（skill_inline_shell_enabled 等） | — | no-op (Phase 2 使用) |
| §7 安全设计（分层防御矩阵） | Task 1-6 内嵌 | covered |
| §8 测试计划（Phase 1 单元测试） | Task 1-7 | covered |

- [ ] **2. Placeholder scan:** 计划中无 TODO/TBD/deferred-by-dependency。Task 1 使用了临时占位函数，但明确标注了替换任务（Task 2/3/4），且执行时按依赖顺序进行，不会留下死代码。

- [ ] **3. No phantom tasks (binary):** 每个任务都有可验证的代码变更和测试。无 `--allow-empty` 提交，无 "already done in Task N" 体。

- [ ] **4. Dependency soundness:** 所有 `Depends on:` 都指向更早的任务。Task 2/3/4 依赖 Task 1；Task 5 依赖 Task 3/4；Task 6 依赖 Task 1-5；Task 7 依赖 Task 1-6。

- [ ] **5. Caller & build soundness:** Task 1 一次性变更了共享签名 `ListActiveByWorkspace`，并更新了所有 2 个生产调用者（handler.go, runtime.go）和测试调用者。每个任务结束都有 `go vet ./...` 全树类型检查（包含 `_test.go`）。同一签名只在 Task 1 中变更一次。

- [ ] **6. Test-the-risk:**
   - Task 1：DB 模型变更 + AutoMigrate 验证（SQLite + PostgreSQL）
   - Task 2：frontmatter 字段提取行为测试（空 frontmatter、完整 frontmatter）
   - Task 3：平台匹配行为测试（macos→darwin 映射、不匹配平台）
   - Task 4：条件过滤行为测试（子集检查、空条件、部分匹配）
   - Task 5：系统提示组装测试（分类索引格式、空技能）
   - Task 6：handler 集成无独立单元测试，但 Task 7 集成测试覆盖完整 WS 启动路径
   - Task 7：集成测试验证过滤后的技能列表正确影响系统提示

- [ ] **7. Type consistency:**
   - `AgentSkill.ParsePlatforms()` 等命名在 Task 1 中定义，Task 3/4 中使用一致
   - `ListActiveByWorkspace(ctx, workspaceID, availableTools, availableToolsets)` 签名在 Task 1 中定义，Task 6 调用时一致
   - `extractFrontmatterFields` 返回 6 个 string，与 Task 1 中模型字段类型一致
   - `buildEnhancedSkillsPrompt` 接收 `[]models.AgentSkill`，与 Task 1 模型一致

