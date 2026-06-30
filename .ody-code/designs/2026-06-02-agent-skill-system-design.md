# Agent Skill System 增强设计文档

> **目标**: 将 Hermes Agent 技能系统的核心亮点能力移植到 Hermind 现有骨架上
> **审计策略**: Deep
> **实施方式**: 增量增强（保留 Hermind 骨架，移植 Hermes 血肉）
> **日期**: 2026-06-02

---

## §1 目的与范围 `[C:USER]`

### 目的
将 Hermes Agent 技能系统的核心亮点能力移植到 Hermind 现有骨架上，使 Hermind 的技能系统从「静态 Markdown 存储」升级为「动态、智能、可扩展的 Agent 能力单元」。

### In Scope（V1）

| # | 能力 | 来源 | 阶段 |
|---|------|------|------|
| 1 | 数据模型扩展（`platforms`、`requires_tools`、`requires_toolsets`、`fallback_for_tools`、`fallback_for_toolsets`、`config_vars` 独立列）| Hermes `SKILL.md` frontmatter | Phase 1 |
| 2 | 系统提示增强（强制加载语义 + 分类组织 + 条件可见性过滤）| Hermes `prompt_builder.py` | Phase 1 |
| 3 | 平台隔离（`runtime.GOOS` 匹配过滤）| Hermes `skill_utils.py` | Phase 1 |
| 4 | 动态预处理管道（模板变量 `${VAR}`、技能配置注入）| Hermes `skill_preprocessing.py` | Phase 2 |
| 5 | 内联 shell 执行（`!`cmd``，功能开关保护，默认关闭）| Hermes `skill_preprocessing.py` | Phase 2 |
| 6 | 外部技能目录导入（`external_skill_dirs` 配置）| Hermes `skill_utils.py` + `skills_tool.py` | Phase 2 |
| 7 | 两层缓存（进程 LRU + 磁盘快照）| Hermes `prompt_builder.py` | Phase 2 |
| 8 | Curator LLM 审查（伞状合并、三层信号归并）| Hermes `curator.py` | Phase 3 |
| 9 | `.usage.json` 侧车数据持久化（use/view/patch 时间戳 + 状态）| Hermes `skill_usage.py` | Phase 3 |
| 10 | 审计日志（Curator 操作记录）| 新增 | Phase 3 |

### Out of Scope（Deferred）

| 项目 | 原因 | 目标版本 |
|------|------|---------|
| Cron job 引用自动迁移 | Hermind 尚无 Cron job 技能引用机制 | V2 `[C:DEFERRED]` |
| 技能碰撞检测（多个外部目录同名）| 当前外部目录为可选功能，DB unique index 已保证 workspace 内唯一 | V2 `[C:DEFERRED]` |
| 技能热重载（文件系统 watch）| 外部目录导入是显式的，非持续同步 | V2 `[C:DEFERRED]` |
| 社区技能市场（hub install）| 需要额外的发布/签名基础设施 | V3 `[C:DEFERRED]` |

---

## §2 数据模型设计 `[C:USER]`

### 2.1 `AgentSkill` 扩展

```go
type AgentSkill struct {
    ID                  int       `gorm:"primaryKey"`
    WorkspaceID         int       `gorm:"index:idx_ws_skill,unique"`
    Name                string
    Slug                string    `gorm:"index:idx_ws_skill,unique"`
    
    // [C:USER] 从 frontmatter text 中提取为独立列，用于查询过滤
    Platforms           string    `gorm:"type:text;default:''"`       // JSON array: ["linux","macos"]
    RequiresTools       string    `gorm:"type:text;default:''"`       // JSON array
    RequiresToolsets    string    `gorm:"type:text;default:''"`       // JSON array
    FallbackForTools    string    `gorm:"type:text;default:''"`       // JSON array
    FallbackForToolsets string    `gorm:"type:text;default:''"`       // JSON array
    ConfigVars          string    `gorm:"type:text;default:''"`       // JSON: [{"key":"...","description":"...","default":"..."}]
    
    Description         string
    Category            string
    Content             string    `gorm:"type:text"`
    Frontmatter         string    `gorm:"type:text"`
    Status              string    `gorm:"default:'active'"`
    Pinned              bool      `gorm:"default:false"`
    
    UseCount            int       `gorm:"default:0"`
    ViewCount           int       `gorm:"default:0"`
    PatchCount          int       `gorm:"default:0"`
    LastUsedAt          *time.Time
    LastViewedAt        *time.Time
    LastPatchedAt       *time.Time
    
    // [C:USER] 侧车 JSON 列（替代 .usage.json 文件）
    UsageSidecar        string    `gorm:"type:text;default:'{}'"`
    
    CreatedBy           string    `gorm:"default:'user'"`
    CreatedAt           time.Time
    UpdatedAt           time.Time
}
```

### 2.2 `AgentSkillFile`（无变化）

现有模型已满足 `references/`、`templates/`、`scripts/`、`assets/` 四种子目录的支持文件存储。

### 2.3 新增：`CuratorAuditLog` 模型

```go
type CuratorAuditLog struct {
    ID          int       `gorm:"primaryKey"`
    WorkspaceID int       `gorm:"index"`
    SkillID     int       `gorm:"index"`
    SkillSlug   string
    Action      string    // "mark_stale" | "archive" | "reactivate" | "llm_review" | "merge" | "unpin_blocked"
    Detail      string    `gorm:"type:text"`
    CreatedAt   time.Time
}
```

### 2.4 新增/扩展 SystemSetting 键

| Key | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `skill_inline_shell_enabled` | bool | false | `[C:USER]` 全局开关 |
| `skill_external_dirs_enabled` | bool | false | `[C:USER]` 全局开关 |
| `external_skill_dirs` | JSON array | `[]` | `[C:USER]` 外部目录路径列表 |
| `curator_interval_cron` | string | `"0 0 * * *"` | `[C:USER]` Curator 调度 |
| `curator_idle_hours` | int | 2 | `[C:USER]` 空闲等待小时数 |
| `skill_cache_lru_size` | int | 100 | `[C:INFERRED]` 每 workspace 进程缓存条目数 |

---

## §3 架构与数据流 `[C:USER]`

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              UI Layer                                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │ Skill Editor │  │ Skill List  │  │ Curator     │  │ External Dir    │ │
│  │ (Markdown)   │  │ (Workspace) │  │ Reports     │  │ Import Dialog   │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └────────┬────────┘ │
│         │                │                │                  │          │
│         └────────────────┴────────────────┴──────────────────┘          │
│                                    │                                    │
│                                    ▼                                    │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │                     REST API (handlers/agent_skills.go)              │ │
│  │  POST /workspace/:slug/agent-skills          (Create)               │ │
│  │  GET  /workspace/:slug/agent-skills          (List)                 │ │
│  │  GET  /workspace/:slug/agent-skills/:slug    (Get)                  │ │
│  │  PATCH /workspace/:slug/agent-skills/:slug   (Patch)                │ │
│  │  POST /workspace/:slug/agent-skills/import   (Import from dir) [NEW]│ │
│  │  GET  /workspace/:slug/agent-skills/curator  (Curator report) [NEW] │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
│                                    │                                    │
└────────────────────────────────────┼────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        Service Layer                                     │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────┐ │
│  │ AgentSkillService   │  │ AgentSkillService   │  │ CuratorService  │ │
│  │ (CRUD + validation) │  │ .BuildSkillPrompt() │  │ (Phase 3)       │ │
│  │                     │  │ .PreprocessContent()│  │                 │ │
│  └─────────────────────┘  └─────────────────────┘  └─────────────────┘ │
│            │                       │                       │            │
│            ▼                       ▼                       ▼            │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │                        Data Access (GORM)                            │ │
│  │  ┌─────────────┐  ┌─────────────────┐  ┌─────────────────────────┐  │ │
│  │  │ agent_skills│  │ agent_skill_files│  │ curator_audit_logs     │  │ │
│  │  │ (extended)  │  │ (unchanged)      │  │ (new)                  │  │ │
│  │  └─────────────┘  └─────────────────┘  └─────────────────────────┘  │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────┼────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        Agent Runtime (WebSocket)                         │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  handler.go:buildSessionRegistry()                                   │ │
│  │    │                                                                 │ │
│  │    ├──► tools.Builder.Build() ──► 工具注册表（含 skill_manage 等）    │ │
│  │    │                                                                 │ │
│  │    └──► system_prompt.go:resolveSystemPrompt() [PHASE 1 插入点]      │ │
│  │           │                                                          │ │
│  │           ├──► AgentSkillService.ListActiveByWorkspace()              │ │
│  │           │      ├── [C:USER] 平台过滤：Platforms × runtime.GOOS       │ │
│  │           │      └── [C:USER] 条件过滤：requires_tools × 可用工具集     │ │
│  │           │                                                          │ │
│  │           └──► 组装增强型系统提示（强制加载语义 + 分类索引）            │ │
│  │                                                                          │
│  │  wsconn.go / session.go runLoop                                       │ │
│  │    │                                                                 │ │
│  │    └──► Agent 调用 skill_view(name)                                  │ │
│  │           │                                                          │ │
│  │           └──► AgentSkillService.GetBySlug()                         │ │
│  │                  ├──► [C:USER] PreprocessContent() [PHASE 2 插入点]   │ │
│  │                  │      ├── 模板变量替换 (${SKILL_DIR}, ${SESSION_ID}) │ │
│  │                  │      ├── 内联 shell 执行 (若开关开启)               │ │
│  │                  │      └── 配置变量注入                               │ │
│  │                  │                                                   │ │
│  │                  └──► [C:USER] 缓存读写 (Phase 2)                     │ │
│  │                         ├── 进程内 LRU (skill slug + hash key)        │ │
│  │                         └── 磁盘快照 (<storage>/cache/skills/)        │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
                                     │
                                     ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                        Background Workers                                │
│  ┌─────────────────────────────────────────────────────────────────────┐ │
│  │  workers.Manager ──► SkillCuratorJob (cron) [PHASE 3 插入点]        │ │
│  │    │                                                                 │ │
│  │    ├──► AgentSkillService.ApplyCuratorTransitions()                  │ │
│  │    │      ├── 时间驱动转换（active→stale→archived）                   │ │
│  │    │      └── [C:USER] LLM 审查（伞状合并，auxiliary 客户端）         │ │
│  │    │                                                                 │ │
│  │    └──► 写入 CuratorAuditLog + <storage>/logs/curator/               │ │
│  └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Phase 1 数据流（系统提示注入路径）

```
[WS Session Start] 
    │
    ▼
resolveSystemPrompt(ws, user, skills)
    │
    ├── 查询 DB: ListActiveByWorkspace(wsID)
    │      │
    │      ├── WHERE status = 'active'
    │      ├── [C:USER] AND (platforms = '' OR platforms JSON_CONTAINS GOOS)
    │      └── ORDER BY category, name
    │
    ├── [C:USER] 条件过滤：遍历 skills，检查 requires_tools ⊆ availableTools
    │      └── 不满足条件的技能跳过（不出现在系统提示中）
    │
    └── 组装增强型提示：
           ┌─────────────────────────────────────────────────────────┐
           │ ## Skills (mandatory)                                   │
           │ Before replying, scan the skills below. If a skill      │
           │ matches, you MUST load it with skill_view(name).        │
           │ ...                                                     │
           │ <available_skills>                                      │
           │   category-1:                                           │
           │     - skill-a: description...                           │
           │     - skill-b: description...                           │
           │   category-2:                                           │
           │     - skill-c: description...                           │
           │ </available_skills>                                    │
           └─────────────────────────────────────────────────────────┘
```

### 3.3 Phase 2 数据流（技能内容加载路径）

```
[Agent calls skill_view("my-skill")]
    │
    ▼
AgentSkillService.GetBySlug("my-skill")
    │
    ├── [C:USER] 缓存查找 (LRU key = workspaceID:slug:contentHash)
    │      ├── HIT ──► 返回缓存的预处理结果
    │      └── MISS ──► 继续
    │
    ├── 查询 DB 获取原始 Content + Frontmatter
    │
    ├── [C:USER] PreprocessContent(content, skillDir, sessionID)
    │      ├── 模板变量替换: ${SKILL_DIR} → absPath, ${SESSION_ID} → uuid
    │      ├── 配置变量注入: 解析 ConfigVars JSON, 查询 SystemSetting
    │      │                注入 "[Skill config: key = value]" 块
    │      └── [C:USER] 内联 shell (若 skill_inline_shell_enabled=true)
    │                   └── exec.Command("bash","-c",cmd) with 10s timeout
    │
    ├── [C:USER] 更新缓存 (LRU + 磁盘快照)
    │
    └── 返回预处理后的内容
```

### 3.4 Phase 3 数据流（Curator 后台路径）

```
[Cron Trigger: 0 0 * * *]
    │
    ▼
SkillCuratorJob.Run()
    │
    ├── 检查 curator_idle_hours: 最近 N 小时是否有活跃会话？
    │      └── 若活跃 ──► 跳过本次运行
    │
    ├── Phase A: 纯时间驱动转换（现有逻辑增强）
    │      ├── 遍历 workspace 的所有 agent-created 技能
    │      ├── [C:USER] 跳过 Pinned 技能
    │      ├── 30d 未使用 → stale, 90d 未使用 → archived
    │      └── 写入 CuratorAuditLog
    │
    └── Phase B: LLM 审查（新增）
           │
           ├── 收集候选技能：active + stale, agent-created, 非 pinned
           ├── 构建审查 Prompt（伞状合并指令）
           ├── 调用 Pantheon auxiliary 客户端
           │      ├── 成功 ──► 解析 YAML consolidations/prunings 块
           │      │            ├── 模型声明 absorbed_into ──► 权威记录
           │      │            ├── 合并到现有 umbrella ──► 更新目标技能
           │      │            ├── 创建新 umbrella ──► 创建新 AgentSkill
           │      │            └── 降级为 support files ──► 创建 AgentSkillFile
           │      └── 失败 ──► [C:USER] 降级为仅时间驱动，记录日志
           │
           └── [C:USER] 三层信号归并：
                    1. absorbed_into 声明（最权威）
                    2. YAML consolidations 块
                    3. 启发式审计（检查 skill_manage patch 中是否引用被删技能名）
           
           └── 写入 <storage>/logs/curator/YYYYMMDD-HHMMSS/
                  ├── run.json (机器可读)
                  └── REPORT.md (人类可读)
```

---

## §4 Phase 1 详细设计 `[C:USER]`

### 4.1 平台过滤

```go
// [C:USER] 平台映射表：Hermes 的 PLATFORM_MAP 移植到 Go
var platformMap = map[string]string{
    "macos":   "darwin",
    "linux":   "linux",
    "windows": "windows",
}

// [C:USER] 检查技能是否匹配当前平台
func skillMatchesPlatform(skill *models.AgentSkill) bool {
    platforms := skill.ParsePlatforms()
    if len(platforms) == 0 {
        return true // [C:UPSTREAM] 无 platforms 字段 = 所有平台可用
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

### 4.2 条件可见性过滤

```go
// [C:USER] 基于可用工具集过滤技能
func filterSkillsByConditions(
    skills []models.AgentSkill,
    availableTools map[string]struct{},
    availableToolsets map[string]struct{},
) []models.AgentSkill {
    var filtered []models.AgentSkill
    for _, skill := range skills {
        reqToolsets := skill.ParseRequiresToolsets()
        if !subset(reqToolsets, availableToolsets) {
            continue
        }
        reqTools := skill.ParseRequiresTools()
        if !subset(reqTools, availableTools) {
            continue
        }
        filtered = append(filtered, skill)
    }
    return filtered
}
```

### 4.3 增强型系统提示组装

```go
// [C:UPSTREAM] 移植自 Hermes prompt_builder.py 的强制加载语义
func buildEnhancedSkillsPrompt(skills []models.AgentSkill) string {
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
            b.WriteString(skill.Description)
            b.WriteString("\n")
        }
    }
    b.WriteString("</available_skills>\n\n")
    b.WriteString("Only proceed without loading a skill if genuinely none are relevant to the task.")
    return b.String()
}
```

### 4.4 调用点集成

**文件 1**：`backend/internal/agent/handler.go` ~line 59

```go
// [C:USER] 修改后：先获取可用工具信息，再过滤
availableTools, availableToolsets := buildAvailableToolInfo(sess.toolRegistry)
skills := deps.AgentSkillSvc.ListActiveByWorkspace(ctx, ws.ID, availableTools, availableToolsets)
systemPrompt := resolveSystemPrompt(ws, user, skills)
```

**文件 2**：`backend/internal/agent/tools/builder.go`

```go
// [C:USER] 新增：导出当前 registry 的工具名和工具集名
func (b *Builder) AvailableTools() []string { ... }
func (b *Builder) AvailableToolsets() []string { ... }
```

---

## §5 Phase 2 详细设计 `[C:USER]`

### 5.1 动态预处理管道

```go
// [C:UPSTREAM] 模板变量替换：移植自 Hermes substitute_template_vars()
func substituteTemplateVars(content string, skillDir string, sessionID string) string {
    replacer := strings.NewReplacer(
        "${SKILL_DIR}", skillDir,
        "${SESSION_ID}", sessionID,
        "${WORKSPACE_ID}", "",
    )
    return replacer.Replace(content) // [C:USER] 未知变量保留原样
}

// [C:UPSTREAM] 内联 shell 执行：移植自 Hermes expand_inline_shell()
func expandInlineShell(content string, skillDir string, timeoutSec int) string {
    re := regexp.MustCompile("!`([^`\\n]+)`")
    return re.ReplaceAllStringFunc(content, func(match string) string {
        cmd := re.FindStringSubmatch(match)[1]
        ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
        defer cancel()
        cmdExec := exec.CommandContext(ctx, "bash", "-c", cmd)
        if skillDir != "" {
            cmdExec.Dir = skillDir
        }
        out, err := cmdExec.CombinedOutput()
        if err != nil {
            if ctx.Err() == context.DeadlineExceeded {
                return fmt.Sprintf("[shell timeout after %ds: %s]", timeoutSec, cmd)
            }
            return fmt.Sprintf("[shell error: %v]", err)
        }
        output := strings.TrimRight(string(out), "\n")
        if len(output) > 4000 {
            output = output[:4000] + "...[truncated]"
        }
        return output
    })
}
```

### 5.2 两层缓存

```go
// [C:USER] 进程内 LRU 缓存（每 workspace）
type SkillCache struct {
    lru *lru.Cache
    mu  sync.RWMutex
}

// [C:USER] 磁盘快照缓存
func (svc *AgentSkillService) saveDiskCache(wsID int, slug string, contentHash string, processed string) error {
    dir := filepath.Join(svc.storageDir, "cache", "skills", fmt.Sprintf("ws_%d", wsID))
    _ = os.MkdirAll(dir, 0755)
    cacheFile := filepath.Join(dir, fmt.Sprintf("%s_%s.json", slug, contentHash))
    data, _ := json.Marshal(CachedSkill{Content: processed, ProcessedAt: time.Now()})
    return os.WriteFile(cacheFile, data, 0644)
}
```

### 5.3 外部技能目录导入

```go
// [C:UPSTREAM] 移植自 Hermes 的注入模式扫描
func scanForInjection(content string) []string {
    patterns := []string{
        "(?i)ignore\\s+(all\\s+)?previous\\s+(instructions|prompts|commands)",
        "(?i)disregard\\s+.*\\s+(instructions|prompts)",
        "(?i)system\\s+prompt",
        "(?i)you\\s+are\\s+now\\s+.*mode",
        "(?i)<!--.*ignore.*-->",
    }
    var threats []string
    for _, p := range patterns {
        if matched, _ := regexp.MatchString(p, content); matched {
            threats = append(threats, p)
        }
    }
    return threats
}
```

### 5.4 调用点集成

**文件**：`backend/internal/agent/tools/skill_view.go`

```go
// [C:USER] skill_view 工具处理函数
func handleSkillView(args map[string]interface{}, ctx *ToolContext) (string, error) {
    name := args["name"].(string)
    skill, err := ctx.SkillSvc.GetBySlug(ctx.Ctx, ctx.WorkspaceID, name)
    if err != nil {
        return "", err
    }
    
    contentHash := hashString(skill.Content)
    cacheKey := CacheKey{WorkspaceID: ctx.WorkspaceID, Slug: name, ContentHash: contentHash}
    if cached, ok := ctx.Cache.Get(cacheKey); ok {
        return cached.Content, nil
    }
    
    skillDir := ctx.SkillSvc.ResolveSkillDir(ctx.WorkspaceID, name)
    processed, err := ctx.SkillSvc.PreprocessContent(ctx.Ctx, skill, skillDir, ctx.SessionID)
    if err != nil {
        return "", err
    }
    
    ctx.Cache.Set(cacheKey, CachedSkill{Content: processed, ProcessedAt: time.Now()})
    ctx.SkillSvc.saveDiskCache(ctx.WorkspaceID, name, contentHash, processed)
    ctx.SkillSvc.BumpView(ctx.Ctx, skill.ID)
    ctx.SkillSvc.BumpUse(ctx.Ctx, skill.ID)
    
    return processed, nil
}
```

---

## §6 Phase 3 详细设计 `[C:USER]`

### 6.1 CuratorService 主入口

```go
// [C:USER] 主入口：复用现有 worker 调度，扩展逻辑
func (c *CuratorService) Run(ctx context.Context, wsID int) (*CuratorRunResult, error) {
    result := &CuratorRunResult{Timestamp: time.Now()}
    
    // Phase A: 纯时间驱动转换
    _, _ = c.applyTimeDrivenTransitions(ctx, wsID, result)
    
    // Phase B: LLM 审查
    if c.config.LLMReviewEnabled {
        _, err := c.applyLLMReview(ctx, wsID, result)
        if err != nil {
            mlog.Warn("Curator LLM review failed, falling back to time-driven only", 
                mlog.Err(err), mlog.Int("workspace", wsID))
            result.Errors = append(result.Errors, fmt.Sprintf("LLM review failed: %v", err))
        }
    }
    
    c.saveReport(ctx, wsID, result)
    return result, nil
}
```

### 6.2 时间驱动转换

```go
func (c *CuratorService) applyTimeDrivenTransitions(ctx context.Context, wsID int, result *CuratorRunResult) error {
    skills, _ := c.skillSvc.List(ctx, wsID, true)
    now := time.Now()
    staleCutoff := now.AddDate(0, 0, -c.config.StaleAfterDays)
    archiveCutoff := now.AddDate(0, 0, -c.config.ArchiveAfterDays)
    
    for _, skill := range skills {
        if skill.Pinned {
            continue // [C:USER] Pinned 免疫
        }
        sidecar := skill.ParseUsageSidecar()
        if !sidecar.AgentCreated {
            continue // [C:UPSTREAM] 仅审查 agent-created 技能
        }
        
        result.Checked++
        anchor := sidecar.LastUsedAt
        if anchor == nil {
            anchor = &sidecar.LastViewedAt
        }
        if anchor == nil || anchor.IsZero() {
            anchor = &skill.CreatedAt
        }
        
        switch sidecar.State {
        case "active":
            if anchor.Before(archiveCutoff) {
                c.skillSvc.UpdateStatus(ctx, skill.ID, "archived")
                result.Archived++
                c.audit(ctx, wsID, skill.ID, skill.Slug, "archive", "90 days inactive")
            } else if anchor.Before(staleCutoff) {
                c.skillSvc.UpdateStatus(ctx, skill.ID, "stale")
                result.MarkedStale++
                c.audit(ctx, wsID, skill.ID, skill.Slug, "mark_stale", "30 days inactive")
            }
        case "stale":
            if anchor.Before(archiveCutoff) {
                c.skillSvc.UpdateStatus(ctx, skill.ID, "archived")
                result.Archived++
            } else if anchor.After(staleCutoff) {
                c.skillSvc.UpdateStatus(ctx, skill.ID, "active")
                result.Reactivated++
            }
        }
    }
    return nil
}
```

### 6.3 LLM 审查 Prompt

```go
// [C:UPSTREAM] 移植自 Hermes curator.py
func (c *CuratorService) buildReviewPrompt(skills []models.AgentSkill) string {
    var b strings.Builder
    b.WriteString("You are the Skill Curator. Review the following agent-created skills and decide how to consolidate them.\n\n")
    b.WriteString("Hard rules:\n")
    b.WriteString("1. DO NOT touch bundled or user-created skills.\n")
    b.WriteString("2. DO NOT delete any skill. Archiving is the maximum destructive action.\n")
    b.WriteString("3. DO NOT touch pinned skills.\n")
    b.WriteString("4. DO NOT use usage counters as a reason to skip consolidation.\n\n")
    b.WriteString("```yaml\nconsolidations:\n  - skill: <slug>\n    into: <umbrella-slug>\n    reason: <why>\nprunings:\n  - skill: <slug>\n    action: archive\n    reason: <why>\nabsorbed:\n  - skill: <slug>\n    absorbed_into: <umbrella-slug>\n```\n\n")
    for _, s := range skills {
        b.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", s.Slug, s.Content[:min(500, len(s.Content))]))
    }
    return b.String()
}
```

### 6.4 三层信号归并

```go
// [C:UPSTREAM] 移植自 Hermes curator.py _reconcile_classification()
func (c *CuratorService) reconcileAndApply(ctx context.Context, wsID int, candidates []models.AgentSkill, 
    consolidations []Consolidation, prunings []Pruning, absorbed []Absorbed, result *CuratorRunResult) {
    
    candidateMap := make(map[string]*models.AgentSkill)
    for i := range candidates {
        candidateMap[candidates[i].Slug] = &candidates[i]
    }
    
    // [C:UPSTREAM] 第一层：absorbed_into 声明（最权威）
    for _, a := range absorbed {
        if skill, ok := candidateMap[a.Skill]; ok {
            c.mergeSkill(ctx, wsID, skill, a.AbsorbedInto, "model_declared_absorbed", result)
        }
    }
    
    // [C:UPSTREAM] 第二层：consolidations 块
    for _, cons := range consolidations {
        if skill, ok := candidateMap[cons.Skill]; ok {
            if _, err := c.skillSvc.GetBySlug(ctx, wsID, cons.Into); err == nil {
                c.mergeSkill(ctx, wsID, skill, cons.Into, "model_consolidation", result)
            } else {
                c.archiveSkill(ctx, wsID, skill, "consolidation_target_missing", result)
            }
        }
    }
}
```

---

## §7 安全设计 `[C:USER]`

### 7.1 分层防御矩阵

| 风险层 | 威胁 | 缓解措施 |
|:---|:---|:---|
| **技能内容执行** | 内联 shell 任意代码执行 | `[C:USER]` (1) 全局开关默认 `false`；(2) 10s 超时；(3) cwd 限制在技能目录；(4) 仅 admin 可开启开关 |
| **Prompt Injection** | 恶意 SKILL.md 包含隐藏指令 | `[C:USER]` (1) 导入时 `_scan_context_content()` 扫描；(2) 命中则 `quarantined` 状态，拒绝入库 |
| **数据泄露** | 模板变量 `${ENV_SECRET}` 读取环境变量 | `[C:USER]` 白名单变量名（仅 `SKILL_DIR`、`SESSION_ID`、`WORKSPACE_ID`），未知变量保留原样 |
| **Curator 误删** | LLM 审查错误地建议删除重要技能 | `[C:USER]` (1) 仅归档不删除；(2) Pinned 免疫；(3) 三层信号归并；(4) 审计日志 |
| **配置变量敏感数据** | API key 等写入技能配置 | `[C:USER]` 支持 `sensitive: true` 标记，值通过 AES-GCM 加密存储 |
| **外部目录遍历** | `external_skill_dirs` 指向系统敏感路径 | `[C:USER]` `filepath.Abs` + `filepath.Clean`；禁止包含 `..` 的路径 |

---

## §8 测试计划 `[C:USER]`

### 8.1 单元测试断言清单

| 测试文件 | 断言内容 |
|:---|:---|
| `models/agent_skill_test.go` | `ParsePlatforms()` 正确解析 JSON 数组；空字符串返回空切片 |
| `services/skill_platform_test.go` | `skillMatchesPlatform`：无 platforms 时全匹配；macos→darwin 映射正确 |
| `services/skill_filter_test.go` | `filterSkillsByConditions`：requires_tools 子集测试；工具缺失时正确排除 |
| `services/skill_preprocess_test.go` | 模板变量替换：已知变量替换成功，未知变量保留原样；配置注入默认值填充 |
| `services/skill_shell_test.go` | 内联 shell：超时返回占位文本；非零退出码返回错误占位 |
| `services/skill_cache_test.go` | LRU hit/miss；磁盘快照读写；ContentHash 变化时 cache miss |
| `services/skill_import_test.go` | 路径遍历阻止；注入模式命中时 quarantined；深度限制 |
| `services/curator_time_test.go` | 30d inactive → stale；90d → archived；pinned 免疫；reactivate 路径 |
| `services/curator_llm_test.go` | mock LLM 输出解析：absorbed_into 权威优先；目标不存在时降级 archive |

### 8.2 集成测试断言清单

| 场景 | 验证点 |
|:---|:---|
| **WS 会话启动** | 系统提示包含增强型技能章节；条件过滤后仅显示匹配工具的技能 |
| **skill_view 调用** | Agent 调用 skill_view 返回预处理后的内容；缓存命中时无 DB 查询 |
| **外部目录导入 API** | `POST /workspace/:slug/agent-skills/import` 正确扫描；注入文件被隔离 |
| **Curator 后台运行** | Cron 触发后检查 `CuratorAuditLog` 表有记录；`<storage>/logs/curator/` 有报告文件 |

---

## §9 假设与未验证项目 `[C:USER]`

| # | 假设 | 置信度 | 错误影响 | 验证方式 |
|---|------|--------|----------|----------|
| 1 | Pantheon SDK 的 auxiliary 客户端支持 `Complete()` 调用 | **Medium** | Curator Phase B 不可用 | 检查 `backend/internal/agent/llm_factory.go` |
| 2 | `runtime.GOOS` 值足以覆盖 Hermind 部署场景 | **Medium** | 平台过滤粒度不够 | 检查部署文档中的 OS 分布 |
| 3 | LRU 缓存大小 100/Workspace 不会导致内存问题 | **Medium** | 高 Workspace 数时内存膨胀 | 上线后监控 heap profile |
| 4 | Hermes 的伞状合并 Prompt 模板无需大幅修改即可适配 | **Medium** | Curator LLM 输出格式不匹配 | Phase 3 mock LLM 响应测试 |
| 5 | GORM AutoMigrate 对新增 JSON text 列的默认值兼容 SQLite 和 PostgreSQL | **High** | 迁移失败 | 两种 DB 上运行测试 |
| 6 | 前端现有的 Markdown 编辑器无需修改即可支持 YAML frontmatter | **High** | 需要额外前端工作 | 检查前端技能编辑器代码 |
| 7 | `filepath.Walk` 性能足以处理 ≤100 技能 × 5 目录 | **High** | 导入 API 响应慢 | 集成测试测量 Walk 耗时 |
| 8 | `mlog` 日志轮转不会与 `<storage>/logs/curator/` 冲突 | **High** | 日志文件被意外清理 | 检查 `mlog` 配置 |

---

## §10 开放问题 / 已决议决策 `[C:USER]`

| # | 决策 | 来源 | 维度 |
|---|------|------|------|
| 1 | 审计策略 = Deep | 用户直接选择 | 流程 |
| 2 | 实施方式 = 增量增强（保留骨架，移植血肉） | 用户选择 A | Scope |
| 3 | V1 范围 = 全部 6 项能力（a-f） | 用户回复 "abcdef" | Scope |
| 4 | 数据模型 = 部分结构化（关键字段独立列 + frontmatter 文本保留） | 用户选择 B | Data & State |
| 5 | 动态预处理在 `skill_view` 读取时执行（非系统提示注入时） | 用户确认 | Integration |
| 6 | 系统提示增强在 `resolveSystemPrompt()` 中替换现有格式 | 用户确认 | Integration |
| 7 | Curator LLM 审查复用现有 `workers.NewSkillCuratorJob` | 用户确认 | Integration |
| 8 | 平台隔离在 `ListActiveByWorkspace` 查询时过滤 | 用户确认 | Integration |
| 9 | 外部技能目录通过 `SystemSetting external_skill_dirs` 配置 | 用户确认 | Integration |
| 10 | 两层缓存 = 进程 LRU + 磁盘快照 | 用户确认 | Integration |
| 11 | 内联 shell 失败替换为错误占位文本 | 用户确认 | Error & Degradation |
| 12 | 模板变量无法解析保留原样 | 用户确认 | Error & Degradation |
| 13 | 技能配置变量缺失使用 default 或显示 "(not set)" | 用户确认 | Error & Degradation |
| 14 | Curator LLM 失败降级为纯时间驱动 | 用户确认 | Error & Degradation |
| 15 | 外部目录不可读静默跳过 | 用户确认 | Error & Degradation |
| 16 | 条件过滤后零技能保留占位提示 | 用户确认 | Error & Degradation |
| 17 | 内联 shell 双层防护（开关默认关 + 超时 + cwd 限制） | 用户确认 | Security |
| 18 | 模板变量白名单限制（仅 SKILL_DIR / SESSION_ID / WORKSPACE_ID） | 用户确认 | Security |
| 19 | 外部技能导入前 Prompt Injection Guard 扫描 | 用户确认 | Security |
| 20 | Curator 仅归档不删除，Pinned 免疫，审计日志 | 用户确认 | Security |
| 21 | 敏感配置变量 AES-GCM 加密存储 | 用户确认 | Security |
| 22 | 技能生命周期事件结构化 mlog 记录 | 用户确认 | Observability |
| 23 | Curator 报告存 `<storage>/logs/curator/` + `mlog.Info` | 用户确认 | Observability |
| 24 | 预处理管道 `mlog.Debug` 追踪 | 用户确认 | Observability |
| 25 | 缓存指标记录 hit/miss | 用户确认 | Observability |
| 26 | WebSocket 推送 `skill:updated` / `curator:completed` | 用户确认 | Observability |
| 27 | `skill_inline_shell_enabled` 默认 false | 用户确认 | Operations |
| 28 | `skill_external_dirs_enabled` 默认 false | 用户确认 | Operations |
| 29 | Curator 调度 `0 0 * * *`，空闲 2h 触发 | 用户确认 | Operations |
| 30 | 缓存容量：每 workspace 100 条 LRU，磁盘 10 个快照 | 用户确认 | Operations |
| 31 | 外部目录限制：最多 5 个，深度 3 层，单次 100 技能 | 用户确认 | Operations |
| 32 | 多用户权限：外部目录/admin 配置仅限 admin | 用户确认 | Operations |
| 33 | 升级迁移：GORM AutoMigrate 新增列，无 breaking change | 用户确认 | Operations |
| 34 | 实施分 3 个 Phase，每阶段 1 周 | 用户确认方案 A | Operations |

---

*设计文档完成。进入 Deep Audit Gate。*
