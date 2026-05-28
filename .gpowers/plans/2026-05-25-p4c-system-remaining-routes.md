# P4c: System 文件系统 + 资产(PFP/Logo) + 杂项路由实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 System 模块剩余的 17 条路由，覆盖文件系统浏览、文档移除、PFP/Logo 上传与下载、以及杂项系统接口。

**Architecture:** 新建 FileSystemService 处理本地文件/资产读写；SystemHandler 扩展文件系统、PFP/Logo、杂项路由；AuthService 扩展 PFP 更新方法。

**Tech Stack:** Go 1.25, Gin, GORM, SQLite (test), testify

**范围外（P4d）：** Prompt 预设/变量路由、workspace-chats、export-chats、validate-sql-connection。

---

## 文件结构总览

| 文件 | 操作 | 说明 |
|------|------|------|
| `backend/internal/services/filesystem_service.go` | 新建 | 文件系统/资产读写 Service |
| `backend/internal/services/filesystem_service_test.go` | 新建 | FileSystemService 单元测试 |
| `backend/internal/services/auth_service.go` | 修改 | 添加 UpdatePfp 方法 |
| `backend/internal/handlers/system.go` | 修改 | 新增 17 条路由 handler |
| `backend/cmd/server/main.go` | 修改 | 注入 FileSystemService |
| `backend/tests/integration/system_misc_test.go` | 新建 | 集成测试 |

---

## Task 1: FileSystemService

**Files:**
- Create: `backend/internal/services/filesystem_service.go`
- Create: `backend/internal/services/filesystem_service_test.go`

### Step 1: 创建 FileSystemService

创建 `backend/internal/services/filesystem_service.go`，包含以下方法：
- `NewFileSystemService(storageDir)`
- `ListLocalFiles(folderName)` — 列出 documents 目录内容
- `CreateFolder(folderName)` — 创建文件夹
- `RemoveFolder(folderName)` — 删除文件夹
- `RemoveDocument(docName)` — 删除文档
- `AcceptedDocumentTypes()` — 返回支持的 MIME 类型映射
- `GetDocumentPath(docName)` — 返回文档完整路径
- `SaveFile(folderName, filename, reader)` — 保存文件
- `DetectMIME(filePath)` — 检测 MIME 类型
- `AssetsDir()` — 返回 assets 目录
- `PfpDir()` — 返回 pfp 目录
- `SaveAsset(filename, reader)` — 保存资产文件
- `SavePfp(filename, reader)` — 保存 PFP 文件
- `ReadAsset(assetPath)` — 读取资产文件，返回 (found, data, size, mime, err)
- `RemoveAsset(assetPath)` — 删除资产文件
- `IsWithin(basePath, targetPath)` — 目录穿越防护

具体实现参考 P4 infrastructure plan 中 Task 2 的代码（已完成上述设计）。

### Step 2: 编写单元测试

创建 `backend/internal/services/filesystem_service_test.go`：
- `TestFileSystemService_ListLocalFiles` — 创建文件和文件夹，验证列表
- `TestFileSystemService_AcceptedDocumentTypes` — 验证 MIME 映射
- `TestFileSystemService_DetectMIME` — 验证 MIME 检测
- `TestFileSystemService_AssetOperations` — SaveAsset / ReadAsset / RemoveAsset 完整流程

### Step 3: 编译与测试

```bash
cd backend && go build ./internal/services/... && go test ./internal/services/ -run TestFileSystemService -v
```

Expected: 编译通过，4 tests PASS。

### Step 4: Commit

```bash
git add backend/internal/services/filesystem_service.go backend/internal/services/filesystem_service_test.go
git commit -m "feat(phase4c): add FileSystemService for document and asset management"
```

---

## Task 2: AuthService 扩展 + main.go 注入

**Files:**
- Modify: `backend/internal/services/auth_service.go`
- Modify: `backend/cmd/server/main.go`

### Step 1: AuthService 添加 UpdatePfp

在 `backend/internal/services/auth_service.go` 中添加：

```go
func (s *AuthService) UpdatePfp(ctx context.Context, userID int, pfpFilename *string) error {
	return s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", userID).Update("pfp_filename", pfpFilename).Error
}
```

### Step 2: main.go 注入 FileSystemService

在 `backend/cmd/server/main.go` 中，在 docSvc 创建之前：

```go
	fsSvc := services.NewFileSystemService(cfg.StorageDir)
```

### Step 3: 编译验证

```bash
cd backend && go build ./...
```

Expected: 编译通过。

### Step 4: Commit

```bash
git add backend/internal/services/auth_service.go backend/cmd/server/main.go
git commit -m "feat(phase4c): add AuthService.UpdatePfp and wire FileSystemService in main"
```

---

## Task 3: SystemHandler 扩展 — 文件系统 + 杂项路由

**Files:**
- Modify: `backend/internal/handlers/system.go`
- Modify: `backend/cmd/server/main.go`

### Step 1: 扩展 SystemHandler 结构体

```go
type SystemHandler struct {
	sysSvc     *services.SystemService
	apiKeySvc  *services.APIKeyService
	adminSvc   *services.AdminService
	authSvc    *services.AuthService
	cfg        *config.Config
	fsSvc      *services.FileSystemService
	coll       *collector.Client
	vectorSvc  *services.VectorService
}
```

更新构造函数和 RegisterSystemRoutes 签名，接受 `fsSvc`, `coll`, `vectorSvc`。

### Step 2: 文件系统路由 handler

- `LocalFiles` — `fsSvc.ListLocalFiles("")` → `{localFiles}`
- `AcceptedDocumentTypes` — `coll.AcceptedFileTypes(ctx)` → `{types}` 或 404
- `RemoveDocument` — `fsSvc.RemoveDocument(req.Name)` → 200
- `RemoveDocuments` — 循环 `fsSvc.RemoveDocument(name)` → 200
- `RemoveFolder` — `fsSvc.RemoveFolder(req.Name)` → 200

权限：admin/manager。

### Step 3: 杂项路由 handler

- `Migrate` — 返回 200（前端启动探活）
- `EnvDump` — 非 production 返回 200；production 暂不实现 dump（Node 端调用 dumpENV，Go 端暂 skip）
- `CustomAppName` — `sysSvc.GetSetting("custom_app_name")` → `{customAppName}`
- `UpdateDefaultSystemPrompt` — `sysSvc.SetSetting("default_system_prompt", req.Prompt)` → `{success, message}`
- `SystemVectors` — `vectorSvc.Heartbeat(ctx)` 或 count → `{vectorCount}`
- `DocumentProcessingStatus` — `coll.Online(ctx)` → 200 或 503

### Step 4: 注册路由

```go
	r.GET("/migrate", h.Migrate)
	r.GET("/env-dump", h.EnvDump)
	r.GET("/system/local-files", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.LocalFiles)
	r.GET("/system/accepted-document-types", middleware.ValidatedRequest(authSvc), h.AcceptedDocumentTypes)
	r.DELETE("/system/remove-document", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveDocument)
	r.DELETE("/system/remove-documents", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveDocuments)
	r.DELETE("/system/remove-folder", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveFolder)
	r.GET("/system/custom-app-name", h.CustomAppName)
	r.POST("/system/default-system-prompt", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin"}), h.UpdateDefaultSystemPrompt)
	r.GET("/system/system-vectors", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.SystemVectors)
	r.GET("/system/document-processing-status", middleware.ValidatedRequest(authSvc), h.DocumentProcessingStatus)
```

### Step 5: 更新 main.go 调用

```go
	handlers.RegisterSystemRoutes(api, sysSvc, apiKeySvc, cfg, authSvc, adminSvc, fsSvc, coll, vectorSvc)
```

### Step 6: 编译验证

```bash
cd backend && go build ./...
```

### Step 7: Commit

```bash
git add backend/internal/handlers/system.go backend/cmd/server/main.go
git commit -m "feat(phase4c): add file system and misc system routes"
```

---

## Task 4: SystemHandler 扩展 — PFP/Logo 路由

**Files:**
- Modify: `backend/internal/handlers/system.go`

### Step 1: PFP handler

- `GetPfp` — 校验用户只能看自己的 PFP；读取 `assets/pfp/<filename>`；返回图片数据（Content-Type, Content-Disposition, Content-Length）；找不到返回 204
- `UploadPfp` — multipart 上传；生成 uuid 文件名；保存到 `assets/pfp/`；删除旧 PFP；调用 `authSvc.UpdatePfp` 更新用户记录
- `RemovePfp` — 删除 `assets/pfp/<filename>`；调用 `authSvc.UpdatePfp` 设为空

### Step 2: Logo handler

- `Logo`（改进已有 stub）— 根据 query.theme 选择默认logo；读取自定义logo文件；返回图片数据及 X-Is-Custom-Logo header；找不到返回 204
- `UploadLogo` — multipart 上传；保存到 `assets/`；删除旧 logo；调用 `sysSvc.SetSetting("logo_filename", filename)`
- `RemoveLogo` — 删除当前自定义 logo；调用 `sysSvc.SetSetting("logo_filename", "anything-llm.png")`

### Step 3: 注册路由

```go
	r.GET("/system/pfp/:id", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.GetPfp)
	r.POST("/system/upload-pfp", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.UploadPfp)
	r.DELETE("/system/remove-pfp", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager", "default"}), h.RemovePfp)
	r.GET("/system/logo", h.Logo)
	r.POST("/system/upload-logo", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.UploadLogo)
	r.GET("/system/remove-logo", middleware.ValidatedRequest(authSvc), middleware.FlexUserRoleValid([]string{"admin", "manager"}), h.RemoveLogo)
```

### Step 4: 编译验证

```bash
cd backend && go build ./...
```

### Step 5: Commit

```bash
git add backend/internal/handlers/system.go
git commit -m "feat(phase4c): add PFP and Logo routes"
```

---

## Task 5: 集成测试

**Files:**
- Create: `backend/tests/integration/system_misc_test.go`

### Step 1: 编写集成测试

测试覆盖：
- `TestMigrate` — GET /migrate → 200
- `TestEnvDump` — GET /env-dump → 200
- `TestLocalFiles` — GET /system/local-files → 200，验证返回结构
- `TestAcceptedDocumentTypes` — GET /system/accepted-document-types → 200 或 404（collector mock）
- `TestCustomAppName` — GET /system/custom-app-name → 200
- `TestUpdateDefaultSystemPrompt` — POST /system/default-system-prompt → 200
- `TestSystemVectors` — GET /system/system-vectors → 200
- `TestDocumentProcessingStatus` — GET /system/document-processing-status → 200/503
- `TestRemoveDocument` / `TestRemoveDocuments` / `TestRemoveFolder` — 创建临时文件后删除
- `TestLogo` — GET /system/logo → 204（无自定义logo时）
- `TestPfp` — GET /system/pfp/:id → 204（无PFP时）

### Step 2: 运行测试

```bash
cd backend && go test ./tests/integration/... -v -count=1
```

Expected: 所有测试 PASS，无 P4a/P4b 回归。

### Step 3: Commit

```bash
git add backend/tests/integration/system_misc_test.go
git commit -m "test(phase4c): integration tests for misc system routes"
```

---

## Task 6: 最终回归与推送

### Step 1: 全量测试

```bash
cd backend && go vet ./... && go build ./... && go test ./... -count=1
```

Expected: build clean, 所有测试 PASS。

### Step 2: Push

```bash
git push origin master
```

---

## 自审检查清单

- [ ] **Spec coverage**: P4c 定义的 17 条路由全部实现
- [ ] **Placeholder scan**: 无 TBD/TODO
- [ ] **Type consistency**: handler 签名与 service 方法一致
- [ ] **Security**: IsWithin 目录穿越校验在所有文件操作中生效
- [ ] **测试**: service 单元测试 + 集成测试覆盖主要路径
