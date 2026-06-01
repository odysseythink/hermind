# Context Compression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把上下文压缩能力接入 hermind 的 Agent 与 Regular Chat 两条路径，复用 Pantheon `agent/compression` 引擎，新增持久化/校准/手动端点/跨 thread handoff/per-workspace 覆盖，并把引擎真实缺口上游贡献给 Pantheon。

**Architecture:** 不在 hermind 重新移植引擎。hermind 侧只做薄接入层（`internal/agent/compression/`：model 元数据、factory、持久化、脱敏正则），两条路径分别接入；引擎缺口在 `pantheon/agent/compression/` 上游补齐。摘要持久化到新表 `thread_compactions`，被压缩的 `WorkspaceChat` 行用 `Include=false` 软删。

**Tech Stack:** Go 1.x、GORM、Gin、Pantheon SDK (`github.com/odysseythink/pantheon`)、React（前端设置页）。

**源码位置：** hermind backend = `/Users/ranwei/workspace/go_work/hermes-agent-rewrite/hermind/backend`；上游 Pantheon = `/Users/ranwei/workspace/go_work/pantheon`。

**设计文档：** `.gpowers/designs/2026-06-01-context-compression-design.md`（Deep 审计通过）。

**分期：**
- **阶段 A — 接入主体**（A1–A9）：模型/factory/persistence/model_metadata/redact + Agent 三处接入 + Chat 路径改造 + 全局开关。可独立交付、产生可用功能。
- **阶段 B — hermind 独立扩展**（B1–B3）：§18.1 `/compress` 端点 + §18.4 per-workspace 覆盖（后端字段 + 前端 UI）。
- **阶段 C — 含上游 Pantheon 改动**（C1–C9）：§10 八项上游 + §18.2 跨 thread handoff + §18.3 实时 usage 校准 + §18.5 600s 冷却。

每个 commit 前在对应仓库根目录跑 `go build ./...` 与相关包测试。所有命令默认在 hermind backend 目录，上游任务显式标注 `cd /Users/ranwei/workspace/go_work/pantheon`。

---

## File Structure

**hermind 新增：**
- `internal/models/thread_compaction.go` — ThreadCompaction 模型
- `internal/agent/compression/model_metadata.go` — model→ctxLen 映射 + 查找
- `internal/agent/compression/redact_patterns.go` — 调优脱敏正则集
- `internal/agent/compression/factory.go` — 构造 compressor（分路径配置）
- `internal/agent/compression/persistence.go` — CompactionStore（thread_compactions 读写）
- `internal/agent/compression/*_test.go`

**hermind 修改：**
- `internal/services/db.go` — AutoMigrate + 默认 SystemSetting
- `internal/services/chat_service.go` — buildChatHistory + 压缩判定 + usage 捕获
- `internal/agent/{handler,session,runtime}.go` — pAgent.New 加 WithContextEngine
- `internal/agent/runtime.go` — Deps 加 CompactionStore
- `internal/models/{workspace_thread,workspace}.go` — 新字段
- `internal/dto/workspace.go` — CreateThreadRequest/UpdateWorkspaceRequest 新字段
- `internal/services/{thread_service,workspace_service}.go` — handoff 种子 + 覆盖字段
- `internal/handlers/workspace.go` — /compress 端点
- 前端 workspace 设置页 — 压缩分区

**上游 Pantheon 修改：** `agent/compression/{state,prune,summary,assemble,config,helpers}.go`、`agent/{agent,stream}.go`

---

# 阶段 A — 接入主体

## Task A1: ThreadCompaction 模型 + 迁移

**Files:**
- Create: `internal/models/thread_compaction.go`
- Modify: `internal/services/db.go`（AutoMigrate 列表）
- Test: `internal/models/thread_compaction_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/models/thread_compaction_test.go
package models

import (
	"testing"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestThreadCompactionMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ThreadCompaction{}))
	tid := 7
	row := ThreadCompaction{WorkspaceID: 1, ThreadID: &tid, Summary: "s", UpToChatID: 42}
	require.NoError(t, db.Create(&row).Error)
	var got ThreadCompaction
	require.NoError(t, db.First(&got, row.ID).Error)
	require.Equal(t, 42, got.UpToChatID)
	require.Equal(t, 7, *got.ThreadID)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/models/ -run TestThreadCompactionMigrate -v`
Expected: FAIL（`undefined: ThreadCompaction`）

- [ ] **Step 3: 实现模型**

```go
// internal/models/thread_compaction.go
package models

import "time"

// ThreadCompaction 持久化一份上下文压缩摘要。
// (WorkspaceID, ThreadID) 定位「最新一份摘要」；ThreadID=nil 表示默认工作区会话（无 thread）。
type ThreadCompaction struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	WorkspaceID      int       `gorm:"index:idx_ws_thread,priority:1" json:"workspaceId"`
	ThreadID         *int      `gorm:"index:idx_ws_thread,priority:2" json:"threadId"`
	Summary          string    `json:"summary"`
	UpToChatID       int       `json:"upToChatId"`
	BeforeTokens     int       `json:"beforeTokens"`
	AfterTokens      int       `json:"afterTokens"`
	FallbackUsed     bool      `json:"fallbackUsed"`
	LastPromptTokens int       `json:"lastPromptTokens"` // §18.3 实时 usage 校准用
	CreatedAt        time.Time `json:"createdAt"`
}
```

- [ ] **Step 4: 注册 AutoMigrate**

在 `internal/services/db.go` 的 AutoMigrate 列表末尾（`&models.AgentSkillFile{},` 之后）加：

```go
		&models.AgentSkillFile{},
		&models.ThreadCompaction{},
	)
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd backend && go test ./internal/models/ -run TestThreadCompactionMigrate -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/models/thread_compaction.go internal/models/thread_compaction_test.go internal/services/db.go
git commit -m "feat(compaction): add ThreadCompaction model + migration"
```

---

## Task A2: model→context-length 映射

**Files:**
- Create: `internal/agent/compression/model_metadata.go`
- Test: `internal/agent/compression/model_metadata_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/agent/compression/model_metadata_test.go
package compression

import "testing"

func TestContextLengthFor(t *testing.T) {
	cases := []struct{ model string; override *int; want int }{
		{"claude-3-5-sonnet", nil, 200000},
		{"gpt-4o-2024-08-06", nil, 128000},   // 前缀匹配 gpt-4o
		{"totally-unknown-model", nil, 8192}, // default
	}
	for _, c := range cases {
		if got := ContextLengthFor(c.model, c.override); got != c.want {
			t.Fatalf("%s: got %d want %d", c.model, got, c.want)
		}
	}
	ov := 50000
	if got := ContextLengthFor("claude-3-5-sonnet", &ov); got != 50000 {
		t.Fatalf("override: got %d want 50000", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/agent/compression/ -run TestContextLengthFor -v`
Expected: FAIL（`undefined: ContextLengthFor`）

- [ ] **Step 3: 实现**

```go
// internal/agent/compression/model_metadata.go
package compression

import "strings"

var modelContextLength = map[string]int{
	"gpt-4o": 128000, "gpt-4o-mini": 128000, "gpt-4-turbo": 128000, "gpt-4": 8192,
	"gpt-3.5-turbo": 16385,
	"claude-3-5-sonnet": 200000, "claude-3-5-haiku": 200000,
	"claude-3-opus": 200000, "claude-3-sonnet": 200000, "claude-3-haiku": 200000,
	"gemini-1.5-pro": 1000000, "gemini-1.5-flash": 1000000, "gemini-2.0-flash": 1000000,
	"llama3": 8192, "llama-3.1": 128000,
	"qwen2": 32768, "qwen2.5": 32768,
	"deepseek-chat": 64000, "deepseek-coder": 64000,
	"mixtral-8x7b": 32768,
}

const defaultContextLength = 8192

// ContextLengthFor 解析模型上下文长度。优先级：workspace 覆盖 → 精确 → 最长前缀 → default。
func ContextLengthFor(model string, wsOverride *int) int {
	if wsOverride != nil && *wsOverride > 0 {
		return *wsOverride
	}
	if v, ok := modelContextLength[model]; ok {
		return v
	}
	bestKeyLen, bestVal := 0, defaultContextLength
	for k, v := range modelContextLength {
		if strings.HasPrefix(model, k) && len(k) > bestKeyLen {
			bestKeyLen, bestVal = len(k), v
		}
	}
	return bestVal
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/agent/compression/ -run TestContextLengthFor -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/compression/model_metadata.go internal/agent/compression/model_metadata_test.go
git commit -m "feat(compaction): add model context-length metadata"
```

---

## Task A3: 调优脱敏正则集

**Files:**
- Create: `internal/agent/compression/redact_patterns.go`
- Test: `internal/agent/compression/redact_patterns_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/agent/compression/redact_patterns_test.go
package compression

import (
	"strings"
	"testing"
	"github.com/odysseythink/pantheon/utils/redact"
)

func TestCompactionRedactKeepsSHAAndEmail(t *testing.T) {
	in := "commit a1b2c3d4e5f60718293a4b5c6d7e8f9012345678 by alice@example.com"
	out := redact.With(in, CompactionRedactPatterns)
	if !strings.Contains(out, "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678") {
		t.Fatal("git SHA must NOT be redacted")
	}
	if !strings.Contains(out, "alice@example.com") {
		t.Fatal("email must NOT be redacted")
	}
}

func TestCompactionRedactScrubsSecrets(t *testing.T) {
	for _, in := range []string{"token sk-ant-abcdefghijklmnopqrstuv", "Authorization: Bearer abc123XYZ789defghijklmnop", "api_key=supersecretvalue123"} {
		if out := redact.With(in, CompactionRedactPatterns); !strings.Contains(out, "[REDACTED]") {
			t.Fatalf("secret not redacted: %q -> %q", in, out)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/agent/compression/ -run TestCompactionRedact -v`
Expected: FAIL（`undefined: CompactionRedactPatterns`）

- [ ] **Step 3: 实现**

```go
// internal/agent/compression/redact_patterns.go
package compression

import "regexp"

// CompactionRedactPatterns 是压缩场景的脱敏规则集：保留真正的密钥规则，
// 去掉 Pantheon 默认集中误杀率高的「裸 32-64 hex」(git SHA/MD5) 与 email 规则。
var CompactionRedactPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/=]{16,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`sk-(?:ant-|proj-|live-)?[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`sk-or-v1-[A-Za-z0-9]{32,}`),
	regexp.MustCompile(`(?i)(password|api_key|apikey|token)\s*[:=]\s*["']?[^\s"']{8,}`),
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/agent/compression/ -run TestCompactionRedact -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/compression/redact_patterns.go internal/agent/compression/redact_patterns_test.go
git commit -m "feat(compaction): tuned redaction pattern set (keep SHA/email)"
```

> 注：本集真正注入引擎依赖上游 C6（`CompressionConfig.RedactPatterns`）。阶段 A 先定义并单测；C6 完成后在 factory（A4）里注入。

---

## Task A4: 压缩器 factory

**Files:**
- Create: `internal/agent/compression/factory.go`
- Test: `internal/agent/compression/factory_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/agent/compression/factory_test.go
package compression

import "testing"

func TestFactoryThresholdsPerPath(t *testing.T) {
	a := NewForAgent(nil)
	c := NewForChat(nil)
	if a.Config().Threshold != 0.50 {
		t.Fatalf("agent threshold got %v want 0.50", a.Config().Threshold)
	}
	if c.Config().Threshold != 0.75 {
		t.Fatalf("chat threshold got %v want 0.75", c.Config().Threshold)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/agent/compression/ -run TestFactoryThresholds -v`
Expected: FAIL（`undefined: NewForAgent` / `Config`）

> 前置：Pantheon `DefaultCompressor` 需暴露 `Config() CompressionConfig`。若上游未暴露，本 Task 依赖 C1（accessor）一并加 `Config()`。阶段 A 若想先独立通过，可临时在 factory 里用本地变量记录 threshold 做断言。下面实现假定 C1 已加 `Config()`（推荐先做 C1）。

- [ ] **Step 3: 实现**

```go
// internal/agent/compression/factory.go
package compression

import (
	"time"
	"github.com/odysseythink/pantheon/agent/compression"
	"github.com/odysseythink/pantheon/core"
)

func baseConfig() compression.CompressionConfig {
	return compression.CompressionConfig{
		Enabled: true, ProtectFirstN: 3, ProtectLast: 20,
		AntiThrashEnabled: true, AntiThrashThreshold: 0.10, AntiThrashMaxConsecutive: 2,
		CooldownEnabled: true, CooldownBase: 30 * time.Second, CooldownMax: 600 * time.Second,
		RedactionEnabled: true, ToolPruningEnabled: true, IterativeUpdateEnabled: true,
		RedactPatterns: CompactionRedactPatterns, // 依赖上游 C6
	}.WithDefaults()
}

// NewForAgent 构造 Agent 路径压缩器（更早触发：Threshold 0.50）。
func NewForAgent(lm core.LanguageModel) *compression.DefaultCompressor {
	cfg := baseConfig()
	cfg.Threshold = 0.50
	return compression.NewDefaultCompressor(cfg, lm)
}

// NewForChat 构造 Chat 路径压缩器（更晚触发：Threshold 0.75）。
func NewForChat(lm core.LanguageModel) *compression.DefaultCompressor {
	cfg := baseConfig()
	cfg.Threshold = 0.75
	return compression.NewDefaultCompressor(cfg, lm)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/agent/compression/ -run TestFactoryThresholds -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/compression/factory.go internal/agent/compression/factory_test.go
git commit -m "feat(compaction): per-path compressor factory"
```

---

## Task A5: CompactionStore 持久化

**Files:**
- Create: `internal/agent/compression/persistence.go`
- Test: `internal/agent/compression/persistence_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/agent/compression/persistence_test.go
package compression

import (
	"context"
	"testing"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"hermind/internal/models" // 按实际 module 名调整
)

func newStoreDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ThreadCompaction{}))
	return db
}

func TestStoreSaveLoadLatestPerThread(t *testing.T) {
	db := newStoreDB(t)
	s := NewCompactionStore(db)
	ctx := context.Background()
	tid := 3
	require.NoError(t, s.Save(ctx, models.ThreadCompaction{WorkspaceID: 1, ThreadID: &tid, Summary: "old", UpToChatID: 10}))
	require.NoError(t, s.Save(ctx, models.ThreadCompaction{WorkspaceID: 1, ThreadID: &tid, Summary: "new", UpToChatID: 20}))
	got := s.LoadLatest(ctx, 1, &tid)
	require.NotNil(t, got)
	require.Equal(t, "new", got.Summary) // 取 id 最大

	// thread_id=nil 与具体 thread 互不串
	require.NoError(t, s.Save(ctx, models.ThreadCompaction{WorkspaceID: 1, ThreadID: nil, Summary: "default-ws"}))
	require.Equal(t, "default-ws", s.LoadLatest(ctx, 1, nil).Summary)
	require.Equal(t, "new", s.LoadLatest(ctx, 1, &tid).Summary)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/agent/compression/ -run TestStore -v`
Expected: FAIL（`undefined: NewCompactionStore`）

- [ ] **Step 3: 实现**

```go
// internal/agent/compression/persistence.go
package compression

import (
	"context"
	"gorm.io/gorm"
	"hermind/internal/models" // 按实际 module 名调整
)

type CompactionStore struct{ db *gorm.DB }

func NewCompactionStore(db *gorm.DB) *CompactionStore { return &CompactionStore{db: db} }

func (s *CompactionStore) Save(ctx context.Context, row models.ThreadCompaction) error {
	return s.db.WithContext(ctx).Create(&row).Error
}

// LoadLatest 取 (workspaceID, threadID) 下 id 最大的一条；无则返回 nil。
func (s *CompactionStore) LoadLatest(ctx context.Context, workspaceID int, threadID *int) *models.ThreadCompaction {
	q := s.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if threadID != nil {
		q = q.Where("thread_id = ?", *threadID)
	} else {
		q = q.Where("thread_id IS NULL")
	}
	var row models.ThreadCompaction
	if err := q.Order("id DESC").First(&row).Error; err != nil {
		return nil
	}
	return &row
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/agent/compression/ -run TestStore -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/compression/persistence.go internal/agent/compression/persistence_test.go
git commit -m "feat(compaction): CompactionStore persistence layer"
```

---

## Task A6: 全局开关 SystemSetting 默认值 + 帮助函数

**Files:**
- Modify: `internal/services/db.go`（SeedDefaults）
- Create: `internal/agent/compression/toggle.go`
- Test: `internal/agent/compression/toggle_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/agent/compression/toggle_test.go
package compression

import "testing"

func TestResolveEnabled(t *testing.T) {
	tru, fls := true, false
	// per-workspace 覆盖优先
	if !ResolveEnabled(&tru, false) { t.Fatal("ws override true must win over global false") }
	if ResolveEnabled(&fls, true) { t.Fatal("ws override false must win over global true") }
	// 无覆盖回落全局
	if !ResolveEnabled(nil, true) { t.Fatal("nil override should fall back to global true") }
	if ResolveEnabled(nil, false) { t.Fatal("nil override should fall back to global false") }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/agent/compression/ -run TestResolveEnabled -v`
Expected: FAIL（`undefined: ResolveEnabled`）

- [ ] **Step 3: 实现 + 默认值**

```go
// internal/agent/compression/toggle.go
package compression

// ResolveEnabled 解析压缩开关：per-workspace 覆盖优先，nil 回落全局。
func ResolveEnabled(wsOverride *bool, global bool) bool {
	if wsOverride != nil {
		return *wsOverride
	}
	return global
}
```

在 `internal/services/db.go` SeedDefaults 的 defaults 切片加一行：

```go
		{Key: "vector_db", Value: strPtr("lancedb")},
		{Key: "context_compress_enabled", Value: strPtr("false")}, // §5 默认 OFF (opt-in)
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/agent/compression/ -run TestResolveEnabled -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/agent/compression/toggle.go internal/agent/compression/toggle_test.go internal/services/db.go
git commit -m "feat(compaction): global toggle default + ResolveEnabled helper"
```

---

## Task A7: Chat 路径 buildChatHistory 改造（读摘要 + 增量）

**Files:**
- Modify: `internal/services/chat_service.go:306-326`（buildChatHistory）、`:28-37`（ChatService 结构体 + 构造器加 compactionStore）
- Test: `internal/services/chat_history_compaction_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/services/chat_history_compaction_test.go
package services

import (
	"context"
	"testing"
	"github.com/stretchr/testify/require"
	"hermind/internal/models"
	// 复用本包已有的内存 DB 测试工具（参考 agent_skill_whitelist_test.go 的 newWhitelistTestDB）
)

func TestBuildChatHistoryWithCompaction(t *testing.T) {
	db := newServicesTestDB(t) // 见下方 helper（若已存在则复用）
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}, &models.ThreadCompaction{}))
	// 3 条历史 chat（id 1..3）
	for i := 1; i <= 3; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{WorkspaceID: 1, Prompt: "p", Response: "r", Include: true}).Error)
	}
	// 摘要覆盖到 id<=2
	require.NoError(t, db.Create(&models.ThreadCompaction{WorkspaceID: 1, Summary: "SUM", UpToChatID: 2}).Error)

	s := &ChatService{db: db, compactionStore: compression.NewCompactionStore(db)}
	hist, err := s.buildChatHistory(context.Background(), 1, nil, 20)
	require.NoError(t, err)
	// 期望：1 条摘要合成消息 + 仅 id=3 的一轮(user+assistant) = 3 条
	require.Len(t, hist, 3)
	require.Contains(t, hist[0].Text(), "SUM")
}
```

> 若 `newServicesTestDB` 不存在，在本包测试文件加一个：用 `sqlite.Open(":memory:")` + 必要 AutoMigrate。参考 `internal/services/agent_skill_whitelist_test.go` 的现有写法。

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/services/ -run TestBuildChatHistoryWithCompaction -v`
Expected: FAIL（`compactionStore` 字段不存在 / 行为不符）

- [ ] **Step 3: 改 ChatService 结构体 + 构造器**

`internal/services/chat_service.go` 结构体加字段：

```go
type ChatService struct {
	db              *gorm.DB
	cfg             *config.Config
	vectorSvc       *VectorService
	llmProv         providers.LLMProvider
	embedder        embedder.Embedder
	agentInvoker    AgentInvoker
	reranker        reranker.Reranker
	memInj          *MemoryInjector
	autoTitleSvc    *AutoTitleService
	compactionStore *compression.CompactionStore // 新增
	sysSvc          *SystemService               // 新增：读全局开关
}
```

`NewChatService` 末尾加参数 `compactionStore *compression.CompactionStore, sysSvc *SystemService` 并赋值（同时更新 `cmd/server/main.go` 中的调用点；用 `grep -rn "NewChatService(" backend` 找全部调用并补参）。

- [ ] **Step 4: 改 buildChatHistory**

把 `internal/services/chat_service.go:306` 的 `buildChatHistory` 替换为：

```go
func (s *ChatService) buildChatHistory(ctx context.Context, workspaceID int, threadID *int, limit int) ([]core.Message, error) {
	var comp *models.ThreadCompaction
	if s.compactionStore != nil {
		comp = s.compactionStore.LoadLatest(ctx, workspaceID, threadID)
	}

	var chats []models.WorkspaceChat
	query := s.db.Where("workspace_id = ? AND include = ?", workspaceID, true)
	if threadID != nil {
		query = query.Where("thread_id = ?", *threadID)
	} else {
		query = query.Where("thread_id IS NULL")
	}
	if comp != nil {
		query = query.Where("id > ?", comp.UpToChatID)
	}
	if err := query.Order("id DESC").Limit(limit).Find(&chats).Error; err != nil {
		return nil, err
	}

	history := make([]core.Message, 0, len(chats)*2+1)
	if comp != nil {
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT,
			"[Compressed summary of earlier conversation]\n"+comp.Summary))
	}
	for i := len(chats) - 1; i >= 0; i-- {
		c := chats[i]
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_USER, c.Prompt))
		history = append(history, core.NewTextMessage(core.MESSAGE_ROLE_ASSISTANT, c.Response))
	}
	return history, nil
}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd backend && go test ./internal/services/ -run TestBuildChatHistoryWithCompaction -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/services/chat_service.go internal/services/chat_history_compaction_test.go cmd/server/main.go
git commit -m "feat(compaction): chat history reads persisted summary + incremental tail"
```

---

## Task A8: Chat 路径压缩判定（落库 + 软删）

**Files:**
- Modify: `internal/services/chat_service.go`（buildRAGContext 内，:48 buildChatHistory 调用之后插入压缩块）
- Create: `internal/agent/compression/estimate.go`（token 估算 helper）
- Test: `internal/services/chat_compaction_trigger_test.go`

- [ ] **Step 1: 写 token 估算 helper（导出供 service 用）**

```go
// internal/agent/compression/estimate.go
package compression

import "github.com/odysseythink/pantheon/core"

// EstimateTokens 字符估算（与 Pantheon helpers.go 一致：(len+3)/4），多模态忽略。
func EstimateTokens(msgs []core.Message) int {
	total := 0
	for _, m := range msgs {
		for _, p := range m.Content {
			if tp, ok := p.(core.TextPart); ok {
				total += (len(tp.Text) + 3) / 4
			}
		}
	}
	return total
}
```

- [ ] **Step 2: 写失败测试**

```go
// internal/services/chat_compaction_trigger_test.go
package services

import (
	"context"
	"strings"
	"testing"
	"github.com/stretchr/testify/require"
	"hermind/internal/models"
)

func TestChatCompactionTriggers(t *testing.T) {
	db := newServicesTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}, &models.ThreadCompaction{}))
	// 造足够长的历史以超过 0.75 * ctxLen（用极小 ctxLen override 触发）
	big := strings.Repeat("x ", 2000)
	for i := 0; i < 30; i++ {
		require.NoError(t, db.Create(&models.WorkspaceChat{WorkspaceID: 1, Prompt: big, Response: big, Include: true}).Error)
	}
	ws := &models.Workspace{ID: 1, CompressContextLen: ptrInt(2000)} // 小 ctxLen 强制触发
	s := &ChatService{db: db, compactionStore: compression.NewCompactionStore(db), llmProv: newMockLLMProv()}
	history, err := s.maybeCompressChat(context.Background(), ws, nil, fullHistory(db, 1))
	require.NoError(t, err)
	// 触发后历史变短，且生成了一条 ThreadCompaction
	var n int64
	db.Model(&models.ThreadCompaction{}).Count(&n)
	require.Equal(t, int64(1), n)
	require.Less(t, compression.EstimateTokens(history), 30*compression.EstimateTokens(fullHistory(db, 1)))
}
```

> `newMockLLMProv` 返回一个 `providers.LLMProvider`，其 `LanguageModel()` 返回 mock `core.LanguageModel`（参考 `internal/agent/mockllm_test.go` 的 mock 模式）。`ptrInt`/`fullHistory` 为本测试文件内 helper。

- [ ] **Step 3: 跑测试确认失败**

Run: `cd backend && go test ./internal/services/ -run TestChatCompactionTriggers -v`
Expected: FAIL（`undefined: maybeCompressChat`）

- [ ] **Step 4: 实现 maybeCompressChat + 接入 buildRAGContext**

新增方法：

```go
// internal/services/chat_service.go
func (s *ChatService) compressEnabled(ctx context.Context, ws *models.Workspace) bool {
	global := false
	if s.sysSvc != nil {
		if v, err := s.sysSvc.GetSetting(ctx, "context_compress_enabled"); err == nil {
			global = v == "true"
		}
	}
	return compression.ResolveEnabled(ws.CompressEnabled, global)
}

func (s *ChatService) maybeCompressChat(ctx context.Context, ws *models.Workspace, threadID *int, history []core.Message) ([]core.Message, error) {
	if s.compactionStore == nil || s.llmProv == nil {
		return history, nil
	}
	modelName := ""
	if ws.ChatModel != nil { modelName = *ws.ChatModel }
	ctxLen := compression.ContextLengthFor(modelName, ws.CompressContextLen)
	if compression.EstimateTokens(history) <= int(0.75*float64(ctxLen)) {
		return history, nil
	}

	comp := compression.NewForChat(s.llmProv.LanguageModel())
	comp.UpdateModel(modelName, ctxLen)
	prev := s.compactionStore.LoadLatest(ctx, ws.ID, threadID)
	if prev != nil {
		comp.SetPreviousSummary(prev.Summary) // 依赖 C1 accessor
	}
	before := compression.EstimateTokens(history)
	compressed, err := comp.CompressMessages(ctx, history, "")
	if err != nil {
		mlog.Warning("chat compaction failed: ", err)
		return history, nil // 降级：用原历史
	}
	after := compression.EstimateTokens(compressed)

	// 边界 chat id：当前 (ws, thread) 下最大 id
	var boundaryChatID int
	q := s.db.Model(&models.WorkspaceChat{}).Where("workspace_id = ?", ws.ID)
	if threadID != nil { q = q.Where("thread_id = ?", *threadID) } else { q = q.Where("thread_id IS NULL") }
	q.Select("COALESCE(MAX(id),0)").Scan(&boundaryChatID)

	_ = s.compactionStore.Save(ctx, models.ThreadCompaction{
		WorkspaceID: ws.ID, ThreadID: threadID,
		Summary: comp.PreviousSummary(), UpToChatID: boundaryChatID,
		BeforeTokens: before, AfterTokens: after, FallbackUsed: comp.LastFallbackUsed(),
	})
	upd := s.db.Model(&models.WorkspaceChat{}).Where("workspace_id = ? AND id <= ?", ws.ID, boundaryChatID)
	if threadID != nil { upd = upd.Where("thread_id = ?", *threadID) } else { upd = upd.Where("thread_id IS NULL") }
	upd.Update("include", false)

	mlog.Info("chat compaction: ", before, "->", after, " tokens")
	return compressed, nil
}
```

在 `buildRAGContext`（`chat_service.go:48` 之后、systemPrompt 组装之前）接入：

```go
		history, err = s.buildChatHistory(ctx, ws.ID, threadID, historyLimit)
		if err != nil {
			return "", nil, nil, err
		}
		if s.compressEnabled(ctx, ws) {
			if history, err = s.maybeCompressChat(ctx, ws, threadID, history); err != nil {
				return "", nil, nil, err
			}
		}
```

- [ ] **Step 5: 跑测试确认通过**

Run: `cd backend && go test ./internal/services/ -run TestChatCompactionTriggers -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/compression/estimate.go internal/services/chat_service.go internal/services/chat_compaction_trigger_test.go
git commit -m "feat(compaction): chat path compress-on-threshold + persist + soft-delete"
```

> 依赖说明：本 Task 用到 `comp.SetPreviousSummary` / `PreviousSummary` / `LastFallbackUsed`（C1）与 `RedactPatterns`（C6）。建议执行顺序：先做 C1+C6，再做 A8；或先用 Pantheon `replace` 指向含 accessor 的本地分支。

---

## Task A9: Agent 三处接入 WithContextEngine

**Files:**
- Modify: `internal/agent/runtime.go`（Deps 加 `CompactionStore` + `SysSvc` 已有）、`:182`（pAgent.New）
- Modify: `internal/agent/handler.go:95-98`、`internal/agent/session.go:116-118`
- Test: `internal/agent/agent_compaction_test.go`

- [ ] **Step 1: Deps 加字段**

`internal/agent/runtime.go` Deps 结构体加：

```go
	AgentSkillSvc   services.AgentSkillManager
	CompactionStore *compression.CompactionStore // 新增（import hermind/internal/agent/compression）
```

并在 `cmd/server/main.go` 构造 Deps 处赋值 `CompactionStore: compression.NewCompactionStore(db)`。

- [ ] **Step 2: 写失败测试（e2e 风格，验证接入不 panic 且长会话触发压缩日志）**

```go
// internal/agent/agent_compaction_test.go
package agent_test

import (
	"testing"
	"github.com/stretchr/testify/require"
	"hermind/internal/agent/compression"
)

func TestBuildAgentCompressorCalibrated(t *testing.T) {
	comp := compression.NewForAgent(nil)
	comp.UpdateModel("claude-3-5-sonnet", compression.ContextLengthFor("claude-3-5-sonnet", nil))
	// 校准后 thresholdTokens = 0.5 * 200000 = 100000
	require.False(t, comp.ShouldCompress(50000)) // 低于阈值不压
	require.True(t, comp.ShouldCompress(150000)) // 超阈值压（aux=nil 时 ShouldCompress 仍按 enabled... 见注）
}
```

> 注：Pantheon `ShouldCompress` 在 `aux==nil` 时返回 false（见 state.go）。本断言需 aux 非 nil 才有意义 —— 用一个 mock `core.LanguageModel`（参考 `internal/agent/mockllm_test.go`）替代 nil。请在测试里传入 mock LM。

- [ ] **Step 3: 跑测试确认失败/红**

Run: `cd backend && go test ./internal/agent/ -run TestBuildAgentCompressorCalibrated -v`
Expected: FAIL（断言不符或需 mock）

- [ ] **Step 4: 三处 pAgent.New 接入**

在 `handler.go:95`、`session.go:116`、`runtime.go:182` 三处，把 `pAgent.New(lm, WithRegistry(reg), WithMaxSteps(10))` 改为条件接入（以 handler.go 为例）：

```go
	opts := []pantheonAgent.Option{
		pantheonAgent.WithRegistry(reg),
		pantheonAgent.WithMaxSteps(10),
	}
	if compEnabled { // 由 ResolveEnabled(ws.CompressEnabled, globalSetting) 计算
		modelName := chatModelName(&ws) // 从 ws 配置取
		comp := compression.NewForAgent(lm)
		comp.UpdateModel(modelName, compression.ContextLengthFor(modelName, ws.CompressContextLen))
		if prev := r.deps.CompactionStore.LoadLatest(c.Request.Context(), ws.ID, threadID); prev != nil {
			comp.SetPreviousSummary(prev.Summary)
		}
		sess.compressor = comp
		opts = append(opts, pantheonAgent.WithContextEngine(comp))
	}
	sess.pAgent = pantheonAgent.New(lm, opts...)
```

`session.go` 的 session 结构体加 `compressor *compression.DefaultCompressor` 字段，供 step 后持久化（见 C3 闭环）。三处共用 helper `chatModelName(ws)` 与 `compEnabledFor(ctx, deps, ws)`，放 `internal/agent/compression_wiring.go`。

- [ ] **Step 5: 跑测试确认通过 + 全包构建**

Run: `cd backend && go test ./internal/agent/ -run TestBuildAgentCompressorCalibrated -v && go build ./...`
Expected: PASS + build OK

- [ ] **Step 6: Commit**

```bash
git add internal/agent/ cmd/server/main.go
git commit -m "feat(compaction): wire WithContextEngine into 3 agent entrypoints (calibrated)"
```

> 阶段 A 完成后：全局开关打开即对两条路径生效（摘要持久化的 agent 闭环在 C3 完成；A 阶段 agent 路径已能压缩，只是摘要尚未写回 DB —— 单连接内仍由 Pantheon 内存态迭代）。

---

# 阶段 B — hermind 独立扩展

## Task B1: `/compress` 端点 service 方法

**Files:**
- Modify: `internal/services/chat_service.go`（加 `CompressNow`）
- Test: `internal/services/compress_now_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/services/compress_now_test.go
func TestCompressNowShortHistory(t *testing.T) {
	db := newServicesTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceChat{}, &models.ThreadCompaction{}))
	db.Create(&models.WorkspaceChat{WorkspaceID: 1, Prompt: "p", Response: "r", Include: true})
	s := &ChatService{db: db, compactionStore: compression.NewCompactionStore(db), llmProv: newMockLLMProv()}
	_, err := s.CompressNow(context.Background(), &models.Workspace{ID: 1}, nil, "")
	require.ErrorIs(t, err, ErrNothingToCompress)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/services/ -run TestCompressNow -v`
Expected: FAIL（`undefined: CompressNow`）

- [ ] **Step 3: 实现**

```go
// internal/services/chat_service.go
var ErrNothingToCompress = errors.New("nothing to compress")

type CompactionResult struct {
	Before, After int     `json:"before"`
	SavedPct      float64 `json:"savedPct"`
	Summary       string  `json:"summary"`
	FallbackUsed  bool    `json:"fallbackUsed"`
}

func (s *ChatService) CompressNow(ctx context.Context, ws *models.Workspace, threadID *int, topic string) (*CompactionResult, error) {
	history, err := s.buildChatHistory(ctx, ws.ID, threadID, 1000) // 取全量
	if err != nil { return nil, err }
	if len(history) <= 4 { // head(3)+3+1 的近似下限
		return nil, ErrNothingToCompress
	}
	modelName := ""
	if ws.ChatModel != nil { modelName = *ws.ChatModel }
	ctxLen := compression.ContextLengthFor(modelName, ws.CompressContextLen)
	comp := compression.NewForChat(s.llmProv.LanguageModel())
	comp.UpdateModel(modelName, ctxLen)
	if prev := s.compactionStore.LoadLatest(ctx, ws.ID, threadID); prev != nil {
		comp.SetPreviousSummary(prev.Summary)
	}
	before := compression.EstimateTokens(history)
	compressed, err := comp.CompressMessages(ctx, history, topic) // topic 透传 focusTopic
	if err != nil { return nil, err }
	after := compression.EstimateTokens(compressed)
	if after >= before { // 没省下 → 视为无可压
		return nil, ErrNothingToCompress
	}
	var boundaryChatID int
	bq := s.db.Model(&models.WorkspaceChat{}).Where("workspace_id = ?", ws.ID)
	if threadID != nil { bq = bq.Where("thread_id = ?", *threadID) } else { bq = bq.Where("thread_id IS NULL") }
	bq.Select("COALESCE(MAX(id),0)").Scan(&boundaryChatID)
	_ = s.compactionStore.Save(ctx, models.ThreadCompaction{
		WorkspaceID: ws.ID, ThreadID: threadID, Summary: comp.PreviousSummary(),
		UpToChatID: boundaryChatID, BeforeTokens: before, AfterTokens: after, FallbackUsed: comp.LastFallbackUsed(),
	})
	uq := s.db.Model(&models.WorkspaceChat{}).Where("workspace_id = ? AND id <= ?", ws.ID, boundaryChatID)
	if threadID != nil { uq = uq.Where("thread_id = ?", *threadID) } else { uq = uq.Where("thread_id IS NULL") }
	uq.Update("include", false)
	pct := float64(before-after) / float64(before) * 100
	return &CompactionResult{Before: before, After: after, SavedPct: pct, Summary: comp.PreviousSummary(), FallbackUsed: comp.LastFallbackUsed()}, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/services/ -run TestCompressNow -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/services/chat_service.go internal/services/compress_now_test.go
git commit -m "feat(compaction): CompressNow service for manual /compress"
```

---

## Task B2: `/compress` HTTP 端点

**Files:**
- Modify: `internal/handlers/workspace.go`（加 handler + 路由注册）
- Test: `internal/handlers/compress_endpoint_test.go`

- [ ] **Step 1: 写失败测试**（用 httptest + gin）

```go
// internal/handlers/compress_endpoint_test.go — 断言：短历史→409；正常→200 含 summary
// （参考本包既有 handler 测试的 setup 模式：建内存 DB、注册路由、httptest 发请求）
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/handlers/ -run TestCompressEndpoint -v`
Expected: FAIL（路由 404）

- [ ] **Step 3: 实现 handler + 路由**

```go
// internal/handlers/workspace.go
type compressRequest struct {
	ThreadID *int   `json:"threadId"`
	Topic    string `json:"topic"`
}

func (h *WorkspaceHandler) Compress(c *gin.Context) {
	slug := c.Param("slug")
	ws, err := h.wsSvc.GetBySlug(c.Request.Context(), slug)
	if err != nil { c.JSON(404, gin.H{"error": "workspace not found"}); return }
	var req compressRequest
	_ = c.ShouldBindJSON(&req)
	res, err := h.chatSvc.CompressNow(c.Request.Context(), ws, req.ThreadID, req.Topic)
	switch {
	case errors.Is(err, services.ErrNothingToCompress):
		c.JSON(409, gin.H{"error": "nothing to compress"})
	case err != nil:
		c.JSON(503, gin.H{"error": "summary model unavailable"})
	default:
		c.JSON(200, res)
	}
}
```

路由注册（与现有 `/workspace/:slug/...` 同组，参考 thread.go:134 的 `r.POST` 写法）：

```go
	r.POST("/workspace/:slug/compress", authMiddleware, h.Compress)
```

> `WorkspaceHandler` 需持有 `chatSvc *services.ChatService` 与 `wsSvc`；若当前未注入，在 handler 构造器与 `cmd/server/main.go` 路由装配处补依赖。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/handlers/ -run TestCompressEndpoint -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/workspace.go internal/handlers/compress_endpoint_test.go cmd/server/main.go
git commit -m "feat(compaction): POST /workspace/:slug/compress endpoint"
```

---

## Task B3: per-workspace 覆盖字段 + 前端设置 UI

**Files:**
- Modify: `internal/models/workspace.go`（3 字段）、`internal/dto/workspace.go:9`（UpdateWorkspaceRequest）、`internal/services/workspace_service.go:85-115`（Update）
- Modify: 前端 workspace 设置页组件（React）
- Test: `internal/services/workspace_compress_settings_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/services/workspace_compress_settings_test.go
func TestUpdateWorkspaceCompressFields(t *testing.T) {
	db := newServicesTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Workspace{}))
	db.Create(&models.Workspace{ID: 1, Slug: "w"})
	s := NewWorkspaceService(db, /*...*/)
	en := true; th := 0.6
	require.NoError(t, s.Update(context.Background(), "w", dto.UpdateWorkspaceRequest{CompressEnabled: &en, CompressThreshold: &th}, nil))
	var ws models.Workspace
	db.First(&ws, 1)
	require.NotNil(t, ws.CompressEnabled); require.True(t, *ws.CompressEnabled)
	require.NotNil(t, ws.CompressThreshold); require.Equal(t, 0.6, *ws.CompressThreshold)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/services/ -run TestUpdateWorkspaceCompressFields -v`
Expected: FAIL（字段不存在）

- [ ] **Step 3: 加模型字段 + DTO + Update 逻辑**

`internal/models/workspace.go` 加（§4）：

```go
	CompressEnabled    *bool    `json:"compressEnabled"`
	CompressThreshold  *float64 `json:"compressThreshold"`
	CompressContextLen *int     `json:"compressContextLen"`
```

`internal/dto/workspace.go` UpdateWorkspaceRequest 加：

```go
	CompressEnabled    *bool    `json:"compressEnabled,omitempty"`
	CompressThreshold  *float64 `json:"compressThreshold,omitempty"`
	CompressContextLen *int     `json:"compressContextLen,omitempty"`
```

`internal/services/workspace_service.go:110` 附近 updates map 加：

```go
	if req.CompressEnabled != nil { updates["compress_enabled"] = *req.CompressEnabled }
	if req.CompressThreshold != nil { updates["compress_threshold"] = *req.CompressThreshold }
	if req.CompressContextLen != nil { updates["compress_context_len"] = *req.CompressContextLen }
```

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/services/ -run TestUpdateWorkspaceCompressFields -v`
Expected: PASS

- [ ] **Step 5: 前端设置 UI**

在 workspace 设置页（`grep -rn "openAiHistory\|OpenAiHistory" frontend/src` 定位现有设置表单）新增「上下文压缩」分区：开关三态 select（跟随全局/强制开/强制关 → `null/true/false`）、阈值 number input（0.3–0.95，空=默认）、ctxLen number input（空=映射表）。提交时并入现有 `PUT /workspace/:slug` 请求体的 `compressEnabled/compressThreshold/compressContextLen` 字段。

- [ ] **Step 6: 前端构建 + Commit**

```bash
cd frontend && npm run build   # 或项目既有前端构建命令
cd .. && git add internal/models/workspace.go internal/dto/workspace.go internal/services/workspace_service.go internal/services/workspace_compress_settings_test.go frontend/
git commit -m "feat(compaction): per-workspace override fields + settings UI"
```

---

# 阶段 C — 含上游 Pantheon 改动

> 所有 C* 上游任务在 `cd /Users/ranwei/workspace/go_work/pantheon` 操作，独立提交/PR。完成后 hermind 侧 `go.mod` 升级依赖版本或用 `replace` 指向本地分支验证。

## Task C1: 上游 accessor（PreviousSummary/SetPreviousSummary/LastFallbackUsed/Config）

**Files:**
- Modify: `agent/compression/state.go`、`compressor.go`
- Test: `agent/compression/accessor_test.go`

- [ ] **Step 1: 写失败测试**

```go
// agent/compression/accessor_test.go
package compression

import "testing"

func TestSummaryAccessor(t *testing.T) {
	c := NewDefaultCompressor(DefaultCompressionConfig(), nil)
	c.SetPreviousSummary("hello")
	if c.PreviousSummary() != "hello" { t.Fatal("roundtrip failed") }
	if c.Config().Threshold != 0.5 { t.Fatal("Config() wrong") }
	if c.LastFallbackUsed() { t.Fatal("fallback should be false initially") }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestSummaryAccessor -v`
Expected: FAIL（方法未定义）

- [ ] **Step 3: 实现 accessor**

`agent/compression/state.go` 加（并在 `compressionState` 加 `lastFallbackUsed bool`，在 `generateSummaryWithFallback` 走静态 fallback 分支时置 true）：

```go
func (c *DefaultCompressor) PreviousSummary() string  { return c.state.previousSummary }
func (c *DefaultCompressor) SetPreviousSummary(s string) { c.state.previousSummary = s }
func (c *DefaultCompressor) LastFallbackUsed() bool   { return c.state.lastFallbackUsed }
func (c *DefaultCompressor) Config() CompressionConfig { return c.cfg }
```

`buildStaticFallbackSummary` 调用点（`generateSummaryWithFallback` Level 2）前置 `c.state.lastFallbackUsed = true`；正常 `generateSummary` 成功路径置 false（在 `compressInternal` 开头 `c.state.lastFallbackUsed = false`）。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestSummaryAccessor -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
git add agent/compression/state.go agent/compression/accessor_test.go
git commit -m "feat(compression): expose PreviousSummary/SetPreviousSummary/LastFallbackUsed/Config accessors"
```

---

## Task C2: 上游 — 可注入脱敏集 RedactPatterns

**Files:**
- Modify: `agent/compression/config.go`、`prune.go`、`summary.go`
- Test: `agent/compression/redact_inject_test.go`

- [ ] **Step 1: 写失败测试**

```go
// agent/compression/redact_inject_test.go
func TestInjectableRedactPatterns(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.RedactPatterns = []*regexp.Regexp{regexp.MustCompile(`SECRET\d+`)}
	c := NewDefaultCompressor(cfg, nil)
	got := c.redactText("here is SECRET42 and email a@b.com")
	if strings.Contains(got, "SECRET42") { t.Fatal("custom pattern not applied") }
	if !strings.Contains(got, "a@b.com") { t.Fatal("email should survive custom-only set") }
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -run TestInjectableRedact -v`
Expected: FAIL

- [ ] **Step 3: 实现**

`config.go` `CompressionConfig` 加字段 `RedactPatterns []*regexp.Regexp` (yaml:"-")。加方法：

```go
func (c *DefaultCompressor) redactText(s string) string {
	if len(c.cfg.RedactPatterns) > 0 {
		return redact.With(s, c.cfg.RedactPatterns)
	}
	return redact.String(s)
}
```

`prune.go` `redactToolResult` 与 `summary.go` `generateSummary` 内所有 `redact.String(x)` 改为 `c.redactText(x)`。

- [ ] **Step 4-5: 通过 + Commit**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/compression/ -v`
```bash
git add agent/compression/config.go agent/compression/prune.go agent/compression/summary.go agent/compression/redact_inject_test.go
git commit -m "feat(compression): injectable RedactPatterns via config"
```

---

## Task C3: 上游 — agent loop 回喂 UpdateFromResponse + hermind agent 持久化闭环

**Files:**
- Modify (上游): `agent/agent.go:~290`、`agent/stream.go`（每 step 后回喂 usage）
- Modify (hermind): `internal/agent/session.go`（step 完成回调里持久化摘要 + 发 WS 事件）
- Test: 上游 `agent/usage_feed_test.go`；hermind `internal/agent/agent_persist_test.go`

- [ ] **Step 1: 上游写失败测试 + 实现**

上游：在 `agent.go` 累加 `totalUsage` 处（:288 附近）之后加：

```go
		if a.contextEngine != nil && resp.Usage.PromptTokens > 0 {
			_ = a.contextEngine.UpdateFromResponse(resp.Usage)
		}
```
`stream.go` 同理在每 step 拿到 usage 后回喂。测试断言：连续两 step 后 `compressor` 的 `last_prompt_tokens` 反映最新 usage（可经新增的 `LastPromptTokens()` accessor 或行为验证 ShouldCompress）。

- [ ] **Step 2: 上游通过 + Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
go test ./agent/... && git add agent/agent.go agent/stream.go agent/usage_feed_test.go
git commit -m "feat(agent): feed per-step usage into context engine for accurate threshold"
```

- [ ] **Step 3: hermind 持久化闭环**

在 `internal/agent/session.go` 的 step 完成处（或 conversation 结束回调），若 `sess.compressor != nil` 且 `sess.compressor.PreviousSummary()` 非空且较上次变化：

```go
	if sess.compressor != nil {
		sum := sess.compressor.PreviousSummary()
		if sum != "" && sum != sess.lastPersistedSummary {
			_ = sess.deps.CompactionStore.Save(ctx, models.ThreadCompaction{
				WorkspaceID: sess.wsID, ThreadID: sess.threadID, Summary: sum,
				FallbackUsed: sess.compressor.LastFallbackUsed(),
			})
			sess.lastPersistedSummary = sum
			_ = sess.wc.Send(ServerFrame{Type: "context.compressed", Content: /* JSON before/after/fallback */})
			if sess.compressor.LastFallbackUsed() {
				_ = sess.wc.Send(ServerFrame{Type: "context.compress_warning", Content: "部分上下文压缩失败，已插入静态摘要"})
			}
		}
	}
```

session 结构体加 `lastPersistedSummary string`、`wsID int`、`threadID *int`（若未持有则在 newSession 赋值）。

- [ ] **Step 4: hermind 测试 + Commit**

Run: `cd backend && go test ./internal/agent/ -run TestAgentPersist -v`
```bash
git add internal/agent/session.go internal/agent/agent_persist_test.go
git commit -m "feat(compaction): persist agent summaries + emit context.compressed WS event"
```

---

## Task C4: 上游 — per-tool 摘要模板

**Files:** Modify `agent/compression/prune.go`；Test `agent/compression/per_tool_summary_test.go`

- [ ] **Step 1: 失败测试**

```go
func TestPerToolSummary(t *testing.T) {
	tr := core.ToolResultPart{ToolCallID: "1", Content: []core.ContentParter{core.TextPart{Text: strings.Repeat("x", 300)}}}
	out := summarizeToolResultNamed("terminal", `{"cmd":"npm test"}`, tr)
	if !strings.Contains(out, "terminal") { t.Fatal("must include tool name") }
}
```

- [ ] **Step 2: 失败** → **Step 3: 实现**

`summarizeToolResult` 增加 tool name/args 入参（或新增 `summarizeToolResultNamed`），按 name 分支：terminal→`[terminal] ran '<cmd>' -> <n> lines`、browser_navigate→`[browser_navigate] <url> (<len> chars)`、create_files→`[create_files] wrote <path>`、web_scraping、session_search，其余走通用 `[<name>] <args_preview> (<len> chars)`。`pruneToolResults` 调用点改为传 tool name（从配对的 ToolCallPart 取）。

- [ ] **Step 4-5: 通过 + Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
go test ./agent/compression/ && git add agent/compression/prune.go agent/compression/per_tool_summary_test.go
git commit -m "feat(compression): per-tool one-line result summaries"
```

---

## Task C5: 上游 — 摘要输入截断

**Files:** Modify `agent/compression/helpers.go`（renderTranscript）；Test `agent/compression/transcript_truncate_test.go`

- [ ] **Step 1: 失败测试**：单条 >6000 字符的消息，断言 renderTranscript 输出对该消息截断至 头4000+尾1500 区间且含省略标记。
- [ ] **Step 2: 失败** → **Step 3: 实现**：`renderTranscript` 对每条文本超 6000 字符时保留头 4000 + `"...[truncated]..."` + 尾 1500；tool args 超 1500 截断头 1200。
- [ ] **Step 4-5: 通过 + Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
go test ./agent/compression/ && git add agent/compression/helpers.go agent/compression/transcript_truncate_test.go
git commit -m "feat(compression): truncate per-message transcript input for summarizer"
```

---

## Task C6: 上游 — fallback 模型重试

**Files:** Modify `agent/compression/state.go`（generateSummaryWithFallback Level 1）、`compressor.go`（构造 fallback LM）；Test `agent/compression/fallback_model_test.go`

- [ ] **Step 1: 失败测试**：主 aux 返回 error，配置 `FallbackModel` 非空 + 注入一个 fallbackLM（成功），断言走 fallback 而非静态摘要。
- [ ] **Step 2: 失败** → **Step 3: 实现**：给 `DefaultCompressor` 加可选 `fallbackLM core.LanguageModel`（构造器或 setter 注入）；`generateSummaryWithFallback` Level 1 用 `fallbackLM.Generate` 重试一次；仍失败才走 Level 2 静态 + cooldown。
- [ ] **Step 4-5: 通过 + Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
go test ./agent/compression/ && git add agent/compression/state.go agent/compression/compressor.go agent/compression/fallback_model_test.go
git commit -m "feat(compression): implement fallback-model retry before static fallback"
```

---

## Task C7: 上游 — summary-prefix + 结束标记

**Files:** Modify `agent/compression/assemble.go`；Test `agent/compression/assemble_prefix_test.go`

- [ ] **Step 1: 失败测试**：assemble 后摘要消息含 `summaryPrefix`；当 summary 落为 role=user 时含 `"--- END OF CONTEXT SUMMARY"` 标记。
- [ ] **Step 2: 失败** → **Step 3: 实现**：`assemble` 用 `summaryPrefix` const 包裹摘要内容；当摘要消息 role 为 user 时追加结束标记字符串（贴齐设计 §10#5）。
- [ ] **Step 4-5: 通过 + Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
go test ./agent/compression/ && git add agent/compression/assemble.go agent/compression/assemble_prefix_test.go
git commit -m "feat(compression): use summary prefix + end marker in assembly"
```

---

## Task C8: 上游 — 600s 冷却第三级

**Files:** Modify `agent/compression/state.go`（enterCooldown）；Test `agent/compression/cooldown_tier_test.go`

- [ ] **Step 1: 失败测试**：连续 3 次失败后 `summaryCooldownUntil` 间隔达 ~600s。
- [ ] **Step 2: 失败** → **Step 3: 实现**：`enterCooldown` 用分级 `[]time.Duration{30s,60s,600s}`，idx=min(ineffectiveCount,2)；`DefaultCompressionConfig` 的 `CooldownMax` 提至 600s。
- [ ] **Step 4-5: 通过 + Commit**

```bash
cd /Users/ranwei/workspace/go_work/pantheon
go test ./agent/compression/ && git add agent/compression/state.go agent/compression/config.go agent/compression/cooldown_tier_test.go
git commit -m "feat(compression): 30/60/600s tiered failure cooldown"
```

---

## Task C9: 跨 thread handoff（ParentThreadID + 种子拷贝 + MemoryProvider）

**Files:**
- Modify: `internal/models/workspace_thread.go`（ParentThreadID）、`internal/dto/workspace.go:26`（CreateThreadRequest）、`internal/services/thread_service.go:34`（Create 种子拷贝）
- Create: `internal/agent/compression/memory_provider.go`
- Test: `internal/services/thread_handoff_test.go`、`internal/agent/compression/memory_provider_test.go`

- [ ] **Step 1: 写失败测试**

```go
// internal/services/thread_handoff_test.go
func TestThreadHandoffSeedsParentSummary(t *testing.T) {
	db := newServicesTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.WorkspaceThread{}, &models.ThreadCompaction{}))
	parentID := 1
	db.Create(&models.WorkspaceThread{ID: parentID, WorkspaceID: 1, Slug: "p"})
	db.Create(&models.ThreadCompaction{WorkspaceID: 1, ThreadID: &parentID, Summary: "PARENT-SUM", UpToChatID: 5})
	s := NewThreadService(db, compression.NewCompactionStore(db) /*...*/)
	child, err := s.Create(context.Background(), 1, nil, dto.CreateThreadRequest{Name: "c", Slug: "c", ParentThreadID: &parentID})
	require.NoError(t, err)
	seed := compression.NewCompactionStore(db).LoadLatest(context.Background(), 1, &child.ID)
	require.NotNil(t, seed)
	require.Equal(t, "PARENT-SUM", seed.Summary)
	require.Equal(t, 0, seed.UpToChatID) // 子 thread 从 0 起算
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `cd backend && go test ./internal/services/ -run TestThreadHandoff -v`
Expected: FAIL（`ParentThreadID` 字段/逻辑缺失）

- [ ] **Step 3: 实现**

`internal/models/workspace_thread.go` 加 `ParentThreadID *int gorm:"index"`（已在设计 §4）。
`internal/dto/workspace.go` CreateThreadRequest 加 `ParentThreadID *int json:"parentThreadId"`。
`ThreadService` 加 `compactionStore` 依赖；`Create`（thread_service.go:34）创建 thread 后：

```go
	thread.ParentThreadID = req.ParentThreadID
	// ... 创建 thread ...
	if req.ParentThreadID != nil && s.compactionStore != nil {
		if parent := s.compactionStore.LoadLatest(ctx, workspaceID, req.ParentThreadID); parent != nil {
			_ = s.compactionStore.Save(ctx, models.ThreadCompaction{
				WorkspaceID: workspaceID, ThreadID: &thread.ID,
				Summary: parent.Summary, UpToChatID: 0,
			})
		}
	}
```

`internal/agent/compression/memory_provider.go`：

```go
package compression

import "github.com/odysseythink/pantheon/core"

// CompactionMemoryProvider 实现 Pantheon MemoryProvider，为未来 agent 内部 session
// 切换（subagent）预埋 handoff。V1 主路径是 ThreadService 种子拷贝。
type CompactionMemoryProvider struct{ store *CompactionStore; wsID int }

func NewCompactionMemoryProvider(store *CompactionStore, wsID int) *CompactionMemoryProvider {
	return &CompactionMemoryProvider{store: store, wsID: wsID}
}
func (p *CompactionMemoryProvider) OnPreCompress(msgs []core.Message) ([]core.Message, error) { return msgs, nil }
func (p *CompactionMemoryProvider) OnSessionSwitch(newSessionID, parentSessionID string) error { return nil }
```

memory_provider_test.go：断言 OnPreCompress 透传消息不变、OnSessionSwitch 不报错。

- [ ] **Step 4: 跑测试确认通过**

Run: `cd backend && go test ./internal/services/ -run TestThreadHandoff ./internal/agent/compression/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/workspace_thread.go internal/dto/workspace.go internal/services/thread_service.go internal/agent/compression/memory_provider.go internal/agent/compression/memory_provider_test.go internal/services/thread_handoff_test.go
git commit -m "feat(compaction): cross-thread handoff via ParentThreadID seed copy + MemoryProvider"
```

---

# 收尾验证

## Task FINAL: 全量测试 + 手测

- [ ] **Step 1: 上游 Pantheon 全测**

Run: `cd /Users/ranwei/workspace/go_work/pantheon && go test ./agent/...`
Expected: PASS

- [ ] **Step 2: hermind 全测 + 构建**

Run: `cd backend && go build ./... && go test ./internal/agent/compression/... ./internal/services/... ./internal/agent/... ./internal/handlers/...`
Expected: PASS

- [ ] **Step 3: 手测 — Agent 长会话**

打开全局开关（`context_compress_enabled=true`），跑一个 >50% ctx 的 Agent 会话；观察后端日志 `context compression triggered`，前端 WS 收到 `context.compressed` 事件；断开重连后日志确认 `SetPreviousSummary` 回填。

- [ ] **Step 4: 手测 — Regular Chat 长会话**

跑一个 >75% ctx 的普通聊天；观察 `chat compaction: N->M tokens`；确认前端历史列表仍完整显示（验证 Include=false 软删不影响 UI —— 若 UI 缺失历史，按风险册#1 修正列表查询不带 include 过滤）。

- [ ] **Step 5: 手测 — /compress 端点**

`POST /workspace/<slug>/compress {"topic":"auth flow"}`，断言 200 + summary 偏重 auth 相关；短历史返回 409。

---

## 自检结果（writing-plans self-review）

- **Spec 覆盖**：设计 §1–§18 各项均有对应 Task（A1–A9 接入主体、B1–B3 独立扩展、C1–C9 上游+handoff）。§9 可观测的 WS 事件在 C3 实现；telemetry `compaction_finished` 接入点在 C3 持久化处补一行 `telemetry` 调用（执行时按 `internal/agent/telemetry.go` 现有模式补）。
- **占位符扫描**：核心新组件（模型/factory/persistence/metadata/redact/CompressNow/handoff/上游 accessor）均给出完整代码；B2 handler 测试、C4–C8 上游任务给出断言意图 + 实现要点（这些是对既有 Pantheon 函数的小改，执行者可据现有函数体直接改），无 TBD。
- **类型一致性**：`CompactionStore`/`ThreadCompaction`/`ContextLengthFor`/`NewForAgent`/`NewForChat`/`PreviousSummary`/`SetPreviousSummary`/`LastFallbackUsed`/`Config`/`EstimateTokens`/`ResolveEnabled`/`CompressNow`/`CompactionResult` 跨 Task 命名一致。
- **依赖顺序提示**：A8/A4 用到 C1（accessor）与 C2（RedactPatterns）。推荐执行顺序：**C1 → C2 → A1–A9 → B → C3–C9**；或先用 `go.mod replace` 指向含 C1/C2 的本地 Pantheon 分支，再按 A→B→C 顺序。
