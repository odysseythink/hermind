# v3-B3 — Binary File Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `gpowers:subagent-driven-development` (recommended) or `gpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 PR-AR-6 punt 的 `create-files-agent` 二进制格式补上:docx / pdf / xlsx 三种(pptx 显式 punt,license 风险)。**估算 6h**。

**先决条件**: PR-AR-6 已合(`create_files_agent.go` 现在 txt/md 走通,其它 format 返回 deferred error)。

**Source spec:** `.gpowers/designs/2026-05-27-v3b-user-facing-design.md` §5。

---

## Pre-task

### 现状

```go
// internal/agent/tools/create_files_agent.go (PR-AR-6 实装)
switch args.Format {
case "txt", "md":
    // 已实装
case "docx", "pdf", "pptx", "xlsx":
    return tool.Error(fmt.Sprintf("format %q not yet implemented (deferred to PR-AR-6.1)", args.Format)), nil
}
```

### 库选型(已在 design §5.1 拍定)

| 格式 | 库 | License | go.mod 路径 |
|---|---|---|---|
| docx | `github.com/lukasjarosch/go-docx` | MIT | 添加 |
| pdf | `github.com/go-pdf/fpdf` | MIT | 添加 |
| xlsx | `github.com/xuri/excelize/v2` | BSD-2 | 添加 |
| pptx | (skip) | — | — |

### Args 形状扩展

```go
type createFilesArgs struct {
    Format   string `json:"format"`
    Filename string `json:"filename"`
    Content  any    `json:"content"`  // 不同 format 解释不同
    // 新增 (xlsx 用):
    // content = {"sheets": [{"name":"S1","rows":[[..],[..]]}, ...]}
    // 其它 format:content = "text body"
}
```

### TDD discipline

每 task 一 commit。

---

## Task 0: go.mod + decision artefact

**Files:**
- `backend/go.mod` (MODIFY — add 3 deps)
- `.gpowers/decisions/2026-05-28-pptx-skip.md` (NEW)
- `.gpowers/decisions/2026-05-28-pdf-ascii-only.md` (NEW)

### Steps

- [ ] 加 3 个依赖:
  ```bash
  cd backend
  go get github.com/lukasjarosch/go-docx@latest
  go get github.com/go-pdf/fpdf@latest
  go get github.com/xuri/excelize/v2@latest
  ```

- [ ] 写 `pptx-skip.md`:
  ```markdown
  # create-files-agent — PPTX Format Skipped

  **Date**: 2026-05-28
  **Status**: Adopted
  **Context**: PR-AR-6 把 docx/pdf/pptx/xlsx 都 punt。v3-B3 补三个,但 pptx 无 MIT/BSD-licensed 选项,只能用 unidoc/unioffice(AGPL)。

  **Decision**: 不实现 pptx;`format=pptx` 返回 `tool.Error("pptx format not supported (no permissive-licensed library); please use docx instead")`。

  **Reconsider when**:出现 MIT/Apache-licensed pptx library;或用户明确同意 AGPL。
  ```

- [ ] 写 `pdf-ascii-only.md`:
  ```markdown
  # create-files-agent — PDF ASCII-Only in v3-B3

  **Date**: 2026-05-28
  **Status**: Adopted
  **Context**: go-pdf/fpdf 默认 Helvetica 字体仅支持 Latin-1。中文/日韩/阿拉伯字符需要注册 TTF。

  **Decision**: v3-B3 检测到 non-ASCII 字符就返回 `tool.Error("PDF generation does not yet support non-ASCII text; please use markdown instead")`。

  **Reconsider when**:用户明确需要中文 PDF,follow-up PR 加 TTF 字体注册(可能要 ship 一个开源中文字体文件)。
  ```

- [ ] `go vet ./...` 干净。

### Acceptance

- go.mod 含 3 个新依赖
- 2 个 decision artefact 落地

### Commit

`chore(create-files): add docx/pdf/xlsx deps + decisions`

---

## Task 1: docx 实装

**Files:**
- `backend/internal/agent/tools/create_files_docx.go` (NEW)
- `backend/internal/agent/tools/create_files_docx_test.go` (NEW)

**Tests:**
- `TestDocx_HappyPath_WritesFile`
- `TestDocx_MultiParagraph_Content`
- `TestDocx_ReturnsValidZIP` (docx 是 ZIP 容器,前 4 字节 `PK\x03\x04`)
- `TestDocx_RespectsApprovalGate` (PR-AR-6 已经有 approval,验证依然生效)

### Steps

- [ ] 看 go-docx API 文档(实施时 `go doc github.com/lukasjarosch/go-docx`),按实测调整;若 API 与下面假设不一致,以 SDK 为准。

- [ ] 实装最简路径(白板生成,不基于模板):
  ```go
  package tools

  import (
      "context"
      "fmt"
      "os"
      "strings"

      "github.com/lukasjarosch/go-docx"
  )

  func writeDocxFile(ctx context.Context, dst string, content string, title string) error {
      doc, err := docx.NewBlank()  // SDK 可能叫 docx.New / docx.NewDocument — 用 doc 命令实测
      if err != nil { return fmt.Errorf("docx new: %w", err) }

      // 添加标题
      if title != "" {
          doc.AddHeading(title, 1)
      }
      // 内容按段落分割
      for _, para := range strings.Split(content, "\n\n") {
          if strings.TrimSpace(para) == "" { continue }
          doc.AddParagraph(para)
      }

      f, err := os.Create(dst)
      if err != nil { return fmt.Errorf("docx create file: %w", err) }
      defer f.Close()
      if err := doc.Write(f); err != nil { return fmt.Errorf("docx write: %w", err) }
      return nil
  }
  ```

  > **实施 note**: SDK 实际方法名可能不同(`NewBlank` vs `New` vs `NewDocument`);先 `go doc -all github.com/lukasjarosch/go-docx | head -50` 看 API。

- [ ] 在 `create_files_agent.go` 的 switch 加 case:
  ```go
  case "docx":
      contentStr, _ := args.Content.(string)
      if err := writeDocxFile(ctx, dst, contentStr, args.Filename); err != nil {
          return tool.Error(err.Error()), nil
      }
      tc.Emit("Created docx " + uniqueName)
      return tool.Result(map[string]any{
          "saved_path": dst,
          "filename":   uniqueName,
          "format":     "docx",
      }), nil
  ```

- [ ] 4 个测试,断言文件存在且前 4 字节是 `PK\x03\x04`(ZIP magic)。

### Acceptance

- 4 个测试通过
- 生成的 .docx 用 LibreOffice 能打开(手测)
- 多段内容正确拆分

### Commit

`feat(create-files): docx format via lukasjarosch/go-docx`

---

## Task 2: pdf 实装(ASCII-only)

**Files:**
- `backend/internal/agent/tools/create_files_pdf.go` (NEW)
- `backend/internal/agent/tools/create_files_pdf_test.go` (NEW)

**Tests:**
- `TestPDF_HappyPath_ASCII`
- `TestPDF_NonASCII_ReturnsError`
- `TestPDF_MultiLine_Wraps`
- `TestPDF_ReturnsValidPDFMagic` (`%PDF-` 前缀)
- `TestPDF_LongContent_Paginates`

### Steps

- [ ] 实装:
  ```go
  package tools

  import (
      "context"
      "fmt"
      "os"
      "strings"
      "unicode"

      "github.com/go-pdf/fpdf"
  )

  func writePDFFile(ctx context.Context, dst string, content string, title string) error {
      // ASCII guard
      for _, r := range content {
          if r > unicode.MaxASCII {
              return fmt.Errorf("PDF generation does not yet support non-ASCII text; use markdown instead")
          }
      }

      pdf := fpdf.New("P", "mm", "A4", "")
      pdf.AddPage()
      pdf.SetFont("Helvetica", "B", 14)
      if title != "" {
          pdf.Cell(0, 10, title)
          pdf.Ln(12)
      }
      pdf.SetFont("Helvetica", "", 11)
      for _, line := range strings.Split(content, "\n") {
          pdf.MultiCell(0, 6, line, "", "", false)
      }
      if pdf.Err() {
          return fmt.Errorf("pdf build: %s", pdf.Error())
      }
      f, err := os.Create(dst)
      if err != nil { return fmt.Errorf("pdf create: %w", err) }
      defer f.Close()
      return pdf.Output(f)
  }
  ```

- [ ] 在 switch 加 case:
  ```go
  case "pdf":
      contentStr, _ := args.Content.(string)
      if err := writePDFFile(ctx, dst, contentStr, args.Filename); err != nil {
          return tool.Error(err.Error()), nil
      }
      tc.Emit("Created pdf " + uniqueName)
      return tool.Result(/* ... */), nil
  ```

- [ ] 5 个测试。

### Acceptance

- 5 个测试通过
- Non-ASCII content 返回明确错误,不写文件
- 长内容自动分页(`MultiCell` 内置 auto-page)

### Commit

`feat(create-files): pdf format via go-pdf/fpdf (ASCII-only)`

---

## Task 3: xlsx 实装

**Files:**
- `backend/internal/agent/tools/create_files_xlsx.go` (NEW)
- `backend/internal/agent/tools/create_files_xlsx_test.go` (NEW)

**Tests:**
- `TestXLSX_HappyPath_SingleSheet`
- `TestXLSX_MultipleSheets`
- `TestXLSX_HeaderRowBolded`
- `TestXLSX_EmptyRows_HandledGracefully`
- `TestXLSX_InvalidContent_ReturnsError` (content 不是 sheets 结构)
- `TestXLSX_ReturnsValidZIP` (xlsx 也是 ZIP 容器)

### Steps

- [ ] 实装:
  ```go
  package tools

  import (
      "context"
      "encoding/json"
      "fmt"

      "github.com/xuri/excelize/v2"
  )

  type xlsxContent struct {
      Sheets []xlsxSheet `json:"sheets"`
  }
  type xlsxSheet struct {
      Name string     `json:"name"`
      Rows [][]string `json:"rows"`
  }

  func writeXLSXFile(ctx context.Context, dst string, contentRaw any) error {
      blob, err := json.Marshal(contentRaw)
      if err != nil { return fmt.Errorf("content marshal: %w", err) }
      var c xlsxContent
      if err := json.Unmarshal(blob, &c); err != nil {
          return fmt.Errorf("content must have shape {sheets: [{name, rows}]}: %w", err)
      }
      if len(c.Sheets) == 0 { return fmt.Errorf("at least one sheet required") }

      f := excelize.NewFile()
      defer f.Close()

      // 默认有 "Sheet1",我们要么重命名要么删
      defaultIdx, _ := f.GetSheetIndex("Sheet1")

      headerStyle, _ := f.NewStyle(&excelize.Style{
          Font: &excelize.Font{Bold: true},
      })

      for i, sh := range c.Sheets {
          name := sh.Name
          if name == "" { name = fmt.Sprintf("Sheet%d", i+1) }
          if i == 0 && defaultIdx >= 0 {
              f.SetSheetName("Sheet1", name)
          } else {
              if _, err := f.NewSheet(name); err != nil {
                  return fmt.Errorf("create sheet %s: %w", name, err)
              }
          }
          for rowIdx, row := range sh.Rows {
              for colIdx, cell := range row {
                  axis, err := excelize.CoordinatesToCellName(colIdx+1, rowIdx+1)
                  if err != nil { return err }
                  if err := f.SetCellValue(name, axis, cell); err != nil { return err }
                  if rowIdx == 0 {  // header row bold
                      f.SetCellStyle(name, axis, axis, headerStyle)
                  }
              }
          }
      }

      // 设置 first sheet 为 active
      f.SetActiveSheet(0)
      return f.SaveAs(dst)
  }
  ```

- [ ] 在 switch 加 case(注意 content 不是 string 而是 object):
  ```go
  case "xlsx":
      if err := writeXLSXFile(ctx, dst, args.Content); err != nil {
          return tool.Error(err.Error()), nil
      }
      tc.Emit("Created xlsx " + uniqueName)
      return tool.Result(/* ... */), nil
  ```

- [ ] 调整 `createFilesArgs.Content` 类型从 `string` 到 `any`(因为 xlsx 需要 object;其它格式仍是 string),并在 string-format 路径加 type assertion。

- [ ] 6 个测试。

### Acceptance

- 6 个测试通过
- 单 sheet / 多 sheet 都对
- 表头自动加粗
- 缺少 sheets 字段返回明确错误

### Commit

`feat(create-files): xlsx format via xuri/excelize`

---

## Task 4: 集成 + pptx 显式拒绝

**Files:**
- `backend/internal/agent/tools/create_files_agent.go` (MODIFY)
- `backend/internal/agent/tools/create_files_agent_test.go` (MODIFY)

**Tests:**
- `TestCreateFiles_AllSixFormats_BehaviorMatrix` (table-driven:txt/md/docx/pdf/xlsx 成功;pptx 失败 with 清晰 message)

### Steps

- [ ] 删除 PR-AR-6 stub 行:
  ```go
  case "docx", "pdf", "pptx", "xlsx":
      return tool.Error(fmt.Sprintf("format %q not yet implemented (deferred to PR-AR-6.1)", args.Format)), nil
  ```

  替换为:
  ```go
  case "docx":
      // (实装 in Task 1)
  case "pdf":
      // (实装 in Task 2)
  case "xlsx":
      // (实装 in Task 3)
  case "pptx":
      return tool.Error("pptx format not supported (no permissive-licensed library); please use docx instead"), nil
  ```

- [ ] 实施 `createFilesArgs.Content` 类型改造 — 从 `string` 改为 `any`,所有调用点同步更新。

- [ ] 写表驱动测试覆盖 6 个 format。

- [ ] Full suite green。

### Acceptance

- 5 个 format(txt/md/docx/pdf/xlsx)成功生成可打开文件
- pptx 返回明确错误消息
- PR-AR-6 已有的 txt/md 测试不回归

### Commit

`feat(create-files): consolidate 5 formats + reject pptx with clear message`

---

## Post-PR checklist

- [ ] `go build ./...` 干净
- [ ] `go vet ./...` 干净
- [ ] `go test ./... -race` 100% 绿
- [ ] 手测:让 @agent 用 5 种 format 各生成一次,文件能用对应 office 软件打开
- [ ] go.mod 仅新增 3 个 deps(go-docx / go-pdf/fpdf / excelize),无 transitive 大依赖
- [ ] 2 个 decision artefact

## Risk notes

| Risk | Mitigation |
|---|---|
| go-docx SDK 实际 API 与假设不一致 | Task 1 第 1 步 `go doc -all`,按实测调整 |
| excelize v2 在大数据集 (>100k rows) 慢/OOM | content 50 KiB cap 早就挡住;LLM 输出量小 |
| PDF ASCII-only 让中文用户不满 | decision artefact 已经说明;follow-up 加 TTF 注册 |
| docx 模板模式更通用但本 PR 不做 | 文档化为 future enhancement;现在白板生成够用 |
| xlsx content shape `{sheets: [...]}` 让 LLM 困惑 | Schema 在 `createFilesSchema()` 描述清楚;LLM 看到 schema 会照办 |
| 3 个新依赖让 binary 增大 ~3-5 MiB | 接受;v3-B3 用户感知收益足够 |

## Estimate

| Task | Hours |
|---|---|
| 0. go.mod + decisions | 0.5 |
| 1. docx | 1.5 |
| 2. pdf (ASCII) | 1.5 |
| 3. xlsx | 2.0 |
| 4. 集成 + pptx 拒绝 | 0.5 |
| **Total** | **6.0** (design 估 6h ✓) |

—— end of plan
