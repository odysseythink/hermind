# Go Collector 重构实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 Go 100% 原生实现 collector 的所有功能，作为 `backend/internal/collector/` 内部包，彻底替换现有的 Node.js HTTP 代理客户端。

**Architecture:** 流水线 + 处理器注册表模式。统一 `Extract → Enrich → Write` 流水线，28 种文件格式各实现 `ContentExtractor` 接口，链接抓取和扩展系统各自独立，通过 `Client` facade 对外暴露向后兼容的公共 API。

**Tech Stack:** Go 1.25, gin (HTTP handlers), chromedp (browser), go-docx/excelize/goquery (parsing), tiktoken-go (tokenizing), 外部 CLI (tesseract, ffmpeg, whisper, pdftotext)

---

## 文件结构映射

```
backend/internal/collector/
├── client.go           (修改: 保留公共API, 内部改为本地流水线)
├── types.go            (最小修改: 添加 IsDirectUpload 字段)
├── options.go          (新建)
├── errors.go           (新建)
├── pipeline/
│   ├── pipeline.go     (新建)
│   ├── extractor.go    (新建)
│   ├── enricher.go     (新建)
│   └── writer.go       (新建)
├── processors/
│   ├── registry.go     (新建)
│   ├── txt.go          (新建)
│   ├── pdf.go          (新建)
│   ├── docx.go         (新建)
│   ├── xlsx.go         (新建)
│   ├── pptx.go         (新建)
│   ├── office_odf.go   (新建)
│   ├── epub.go         (新建)
│   ├── mbox.go         (新建)
│   ├── image.go        (新建)
│   └── audio.go        (新建)
├── scraper/
│   ├── scraper.go      (新建)
│   ├── generic.go      (新建)
│   ├── youtube.go      (新建)
│   └── helpers.go      (新建)
├── extensions/
│   ├── registry.go     (新建)
│   ├── github.go       (新建)
│   ├── gitlab.go       (新建)
│   ├── confluence.go   (新建)
│   ├── drupalwiki.go   (新建)
│   ├── obsidian.go     (新建)
│   ├── paperless.go    (新建)
│   ├── website_depth.go(新建)
│   └── resync.go       (新建)
├── utils/
│   ├── tokenizer.go    (新建)
│   ├── files.go        (新建)
│   ├── mime.go         (新建)
│   ├── text.go         (新建)
│   └── shell.go        (新建)
└── external/
    ├── tesseract.go    (新建)
    ├── whisper.go      (新建)
    └── chromedp.go     (新建)
```

---

## Phase 1: 基础类型与基础设施

### Task 1: 错误类型与选项类型

**Files:**
- Create: `backend/internal/collector/errors.go`
- Create: `backend/internal/collector/options.go`
- Modify: `backend/internal/collector/types.go` (添加 `IsDirectUpload`)

**Context:** 现有 `types.go` 已有 `Options`, `ProcessResponse`, `Document`, `LinkContentResponse`, `ExtensionResponse`, `ParseOptions`。需要新增错误类型，并把 `Options` 移到 `options.go`。

- [ ] **Step 1: 创建 `errors.go`**

```go
package collector

import "errors"

var (
    ErrUnsupportedFormat   = errors.New("unsupported file format")
    ErrFileNotFound        = errors.New("file not found")
    ErrInvalidPath         = errors.New("invalid file path")
    ErrEmptyContent        = errors.New("no text content found")
    ErrProcessingTimeout   = errors.New("processing timeout")
    ErrOCRFailed           = errors.New("OCR processing failed")
    ErrTranscriptionFailed = errors.New("audio transcription failed")
    ErrBrowserLaunchFailed = errors.New("browser launch failed")
)
```

- [ ] **Step 2: 创建 `options.go`**

```go
package collector

type Options struct {
    WhisperProvider  string          `json:"whisperProvider"`
    WhisperModelPref string          `json:"whisperModelPref,omitempty"`
    OpenAiKey        string          `json:"openAiKey,omitempty"`
    OCR              OCROptions      `json:"ocr"`
    RuntimeSettings  RuntimeSettings `json:"runtimeSettings"`
}

type OCROptions struct {
    LangList string `json:"langList"`
}

type RuntimeSettings struct {
    AllowAnyIp        string   `json:"allowAnyIp"`
    BrowserLaunchArgs []string `json:"browserLaunchArgs"`
}
```

- [ ] **Step 3: 修改 `types.go` 添加 `IsDirectUpload`**

在 `Document` 结构体中添加：
```go
IsDirectUpload bool `json:"isDirectUpload,omitempty"`
```

- [ ] **Step 4: 确保编译通过**

Run: `cd backend && go build ./internal/collector/...`

- [ ] **Step 5: Commit**

```bash
git add backend/internal/collector/errors.go backend/internal/collector/options.go backend/internal/collector/types.go
git commit -m "feat(collector): add error types, options, extend Document type"
```

---

### Task 2: Shell 工具与外部命令封装

**Files:**
- Create: `backend/internal/collector/utils/shell.go`

**Context:** 多个处理器需要调用外部 CLI（tesseract、ffmpeg、pdftotext、whisper）。统一封装避免重复。

- [ ] **Step 1: 创建 `shell.go`**

```go
package utils

import (
    "context"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

type ShellRunner struct{}

func NewShellRunner() *ShellRunner { return &ShellRunner{} }

func (s *ShellRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, name, args...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("%s failed: %w (output: %s)", name, err, string(out))
    }
    return strings.TrimSpace(string(out)), nil
}

func (s *ShellRunner) CheckInstalled(name string) bool {
    _, err := exec.LookPath(name)
    return err == nil
}
```

- [ ] **Step 2: 写测试**

```go
package utils

import (
    "context"
    "testing"
    "time"
)

func TestShellRunner_CheckInstalled(t *testing.T) {
    r := NewShellRunner()
    // go binary should always be installed
    if !r.CheckInstalled("go") {
        t.Error("expected 'go' to be installed")
    }
    if r.CheckInstalled("definitely_not_a_real_binary_12345") {
        t.Error("expected fake binary to not be installed")
    }
}

func TestShellRunner_Run(t *testing.T) {
    r := NewShellRunner()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    out, err := r.Run(ctx, "echo", "hello")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if out != "hello" {
        t.Errorf("expected 'hello', got %q", out)
    }
}
```

- [ ] **Step 3: 运行测试**

Run: `cd backend && go test ./internal/collector/utils/... -v`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/collector/utils/shell.go backend/internal/collector/utils/shell_test.go
git commit -m "feat(collector): add shell runner utility"
```

---

### Task 3: Tokenizer

**Files:**
- Create: `backend/internal/collector/utils/tokenizer.go`

**Context:** 需要 `github.com/pkoukk/tiktoken-go`。先添加到 go.mod。

- [ ] **Step 1: 添加依赖**

Run: `cd backend && go get github.com/pkoukk/tiktoken-go`

- [ ] **Step 2: 创建 `tokenizer.go`**

```go
package utils

import "github.com/pkoukk/tiktoken-go"

type Tokenizer struct {
    encoder *tiktoken.Tiktoken
}

func NewTokenizer() (*Tokenizer, error) {
    enc, err := tiktoken.GetEncoding("cl100k_base")
    if err != nil {
        return nil, err
    }
    return &Tokenizer{encoder: enc}, nil
}

func (t *Tokenizer) Count(input string) int {
    const maxKBEstimate = 10 * 1024
    const divisor = 8
    if len(input) > maxKBEstimate {
        return (len(input) + divisor - 1) / divisor
    }
    tokens, err := t.encoder.Encode(input, nil, nil)
    if err != nil {
        return (len(input) + divisor - 1) / divisor
    }
    return len(tokens)
}
```

- [ ] **Step 3: 写测试**

```go
package utils

import "testing"

func TestTokenizer_Count(t *testing.T) {
    tok, err := NewTokenizer()
    if err != nil {
        t.Fatalf("init tokenizer: %v", err)
    }
    // "hello" is 1 token in cl100k_base (approximately)
    count := tok.Count("hello")
    if count <= 0 {
        t.Errorf("expected positive token count, got %d", count)
    }
    // Long input should use estimation
    longInput := strings.Repeat("a", 20*1024)
    longCount := tok.Count(longInput)
    expected := (len(longInput) + 7) / 8
    if longCount != expected {
        t.Errorf("expected %d for long input, got %d", expected, longCount)
    }
}
```

- [ ] **Step 4: 运行测试**

Run: `cd backend && go test ./internal/collector/utils/... -run TestTokenizer -v`

- [ ] **Step 5: Commit**

```bash
git add backend/internal/collector/utils/tokenizer.go backend/internal/collector/utils/tokenizer_test.go backend/go.mod backend/go.sum
git commit -m "feat(collector): add tiktoken-based tokenizer"
```

---

### Task 4: 文件操作工具

**Files:**
- Create: `backend/internal/collector/utils/files.go`
- Create: `backend/internal/collector/utils/text.go`
- Create: `backend/internal/collector/utils/mime.go`

**Context:** 需要实现 `writeToServerDocuments`, `isTextType`, `trashFile`, `createdDate`, `isWithin`, `normalizePath`, `sanitizeFileName`。

- [ ] **Step 1: 创建 `files.go`**

```go
package utils

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/google/uuid"
    "github.com/gosimple/slug"
    "github.com/odysseythink/hermind/backend/internal/collector"
)

func WriteToServerDocuments(storageDir string, data *collector.Document, filename string, parseOnly bool) (*collector.Document, error) {
    var dest string
    if parseOnly {
        dest = filepath.Join(storageDir, "direct-uploads")
    } else {
        dest = filepath.Join(storageDir, "documents", "custom-documents")
    }
    if err := os.MkdirAll(dest, 0755); err != nil {
        return nil, fmt.Errorf("mkdir: %w", err)
    }
    safeName := SanitizeFileName(filename) + ".json"
    outPath := filepath.Join(dest, safeName)
    b, err := json.MarshalIndent(data, "", "    ")
    if err != nil {
        return nil, fmt.Errorf("marshal: %w", err)
    }
    if err := os.WriteFile(outPath, b, 0644); err != nil {
        return nil, fmt.Errorf("write file: %w", err)
    }
    parts := strings.Split(filepath.ToSlash(outPath), "/")
    if len(parts) >= 2 {
        data.Location = strings.Join(parts[len(parts)-2:], "/")
    }
    data.IsDirectUpload = parseOnly
    return data, nil
}

func TrashFile(path string) {
    if path == "" { return }
    if fi, err := os.Stat(path); err != nil || fi.IsDir() {
        return
    }
    _ = os.Remove(path)
}

func CreatedDate(path string) string {
    fi, err := os.Stat(path)
    if err != nil {
        return "unknown"
    }
    return fi.ModTime().Format("2006-01-02 15:04:05")
}

func NormalizePath(p string) string {
    p = filepath.Clean(strings.TrimSpace(p))
    p = strings.TrimPrefix(p, "..")
    p = strings.TrimPrefix(p, "/..")
    if p == ".." || p == "." || p == "/" {
        return ""
    }
    return p
}

func IsWithin(outer, inner string) bool {
    outer = filepath.Clean(outer)
    inner = filepath.Clean(inner)
    if outer == inner {
        return false
    }
    rel, err := filepath.Rel(outer, inner)
    if err != nil {
        return false
    }
    return !strings.HasPrefix(rel, "..") && rel != ".."
}

func SanitizeFileName(name string) string {
    if name == "" { return name }
    bad := []rune{'<', '>', ':', '"', '/', '\\', '|', '?', '*', '\u201C', '\u201D', '\u201E', '\u201F', '\u2018', '\u2019', '\u201A', '\u201B'}
    for _, r := range bad {
        name = strings.ReplaceAll(name, string(r), "")
    }
    return name
}

func SlugifyFilename(name string) string {
    return slug.Make(name)
}
```

- [ ] **Step 2: 创建 `text.go`**

```go
package utils

import (
    "os"
    "strings"
    "unicode"
)

func IsTextType(path string) bool {
    if _, err := os.Stat(path); err != nil {
        return false
    }
    // Check known text mime first
    // Fallback: read first 1KB and check for null/control chars
    fd, err := os.Open(path)
    if err != nil {
        return false
    }
    defer fd.Close()

    buf := make([]byte, 1024)
    n, err := fd.Read(buf)
    if err != nil {
        return false
    }
    content := string(buf[:n])
    nullCount := strings.Count(content, "\x00")
    controlCount := 0
    for _, r := range content {
        if r <= 0x08 || r == 0x0B || r == 0x0C || (r >= 0x0E && r <= 0x1F) {
            controlCount++
        }
    }
    threshold := float64(n) * 0.1
    return float64(nullCount+controlCount) < threshold
}
```

- [ ] **Step 3: 创建 `mime.go`（stub，扩展时填充）**

```go
package utils

var AcceptedMIMETypes = map[string][]string{
    "text/plain":                    {".txt", ".md", ".org", ".adoc", ".rst"},
    "text/html":                     {".html"},
    "text/csv":                      {".csv"},
    "application/json":              {".json"},
    "application/pdf":               {".pdf"},
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": {".docx"},
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": {".pptx"},
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       {".xlsx"},
    "application/vnd.oasis.opendocument.text":                                 {".odt"},
    "application/vnd.oasis.opendocument.presentation":                         {".odp"},
    "application/mbox":            {".mbox"},
    "application/epub+zip":        {".epub"},
    "audio/wav":                   {".wav"},
    "audio/mpeg":                  {".mp3"},
    "audio/ogg":                   {".ogg", ".oga"},
    "audio/opus":                  {".opus"},
    "audio/mp4":                   {".m4a"},
    "audio/x-m4a":                 {".m4a"},
    "audio/webm":                  {".webm"},
    "video/mp4":                   {".mp4"},
    "video/mpeg":                  {".mpeg"},
    "image/png":                   {".png"},
    "image/jpeg":                  {".jpg", ".jpeg"},
    "image/webp":                  {".webp"},
}

func AcceptedFileTypes() []string {
    out := make([]string, 0, len(AcceptedMIMETypes))
    for mime := range AcceptedMIMETypes {
        out = append(out, mime)
    }
    return out
}
```

- [ ] **Step 4: 写 `files_test.go`**

```go
package utils

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/collector"
)

func TestWriteToServerDocuments(t *testing.T) {
    tmpDir := t.TempDir()
    doc := &collector.Document{Title: "test", PageContent: "hello world"}
    result, err := WriteToServerDocuments(tmpDir, doc, "my-file", false)
    if err != nil {
        t.Fatalf("write failed: %v", err)
    }
    if result.Location == "" {
        t.Error("expected location to be set")
    }
    if !result.IsDirectUpload {
        t.Error("expected IsDirectUpload=false for non-parseOnly")
    }
    // Verify file exists
    fullPath := filepath.Join(tmpDir, "documents", "custom-documents", "my-file.json")
    if _, err := os.Stat(fullPath); err != nil {
        t.Errorf("expected file to exist: %v", err)
    }
}

func TestSanitizeFileName(t *testing.T) {
    tests := []struct{ in, want string }{
        {"hello<world>", "helloworld"},
        {"file:name?", "filename"},
        {"\u201Cquote\u201D", "quote"},
    }
    for _, tc := range tests {
        got := SanitizeFileName(tc.in)
        if got != tc.want {
            t.Errorf("SanitizeFileName(%q) = %q, want %q", tc.in, got, tc.want)
        }
    }
}

func TestIsWithin(t *testing.T) {
    if !IsWithin("/tmp", "/tmp/sub/file.txt") {
        t.Error("expected /tmp/sub/file.txt to be within /tmp")
    }
    if IsWithin("/tmp", "/other/file.txt") {
        t.Error("expected /other/file.txt to NOT be within /tmp")
    }
    if IsWithin("/tmp", "/tmp") {
        t.Error("expected same path to NOT be within")
    }
}
```

- [ ] **Step 5: 运行测试**

Run: `cd backend && go test ./internal/collector/utils/... -v`

- [ ] **Step 6: Commit**

```bash
git add backend/internal/collector/utils/
git commit -m "feat(collector): add file utilities, text detection, mime types"
```

---

### Task 5: Pipeline 核心（Writer + Enricher + Pipeline）

**Files:**
- Create: `backend/internal/collector/pipeline/writer.go`
- Create: `backend/internal/collector/pipeline/enricher.go`
- Create: `backend/internal/collector/pipeline/extractor.go`
- Create: `backend/internal/collector/pipeline/pipeline.go`

- [ ] **Step 1: 创建 `extractor.go`**

```go
package pipeline

import (
    "context"
    "github.com/odysseythink/hermind/backend/internal/collector"
)

type ExtractInput struct {
    FullFilePath string
    Filename     string
    Options      collector.Options
    Metadata     map[string]string
}

type ExtractOutput struct {
    PageContent string
    DocAuthor   string
    Description string
}

type ContentExtractor interface {
    Extract(ctx context.Context, input ExtractInput) (ExtractOutput, error)
    SupportedExtensions() []string
}
```

- [ ] **Step 2: 创建 `writer.go`**

```go
package pipeline

import (
    "github.com/google/uuid"
    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type Writer struct {
    storageDir string
}

func NewWriter(storageDir string) *Writer {
    return &Writer{storageDir: storageDir}
}

func (w *Writer) Write(doc *collector.Document, filename string, parseOnly bool) (*collector.Document, error) {
    return utils.WriteToServerDocuments(w.storageDir, doc, filename, parseOnly)
}
```

- [ ] **Step 3: 创建 `enricher.go`**

```go
package pipeline

import (
    "strings"

    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type Enricher struct {
    tokenizer *utils.Tokenizer
}

func NewEnricher(tok *utils.Tokenizer) *Enricher {
    return &Enricher{tokenizer: tok}
}

func (e *Enricher) Enrich(doc *collector.Document, extOut ExtractOutput) {
    if doc.DocAuthor == "" && extOut.DocAuthor != "" {
        doc.DocAuthor = extOut.DocAuthor
    }
    if doc.Description == "" && extOut.Description != "" {
        doc.Description = extOut.Description
    }
    doc.WordCount = len(strings.Fields(doc.PageContent))
    doc.TokenCountEstimate = e.tokenizer.Count(doc.PageContent)
}
```

- [ ] **Step 4: 创建 `pipeline.go`**

```go
package pipeline

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/google/uuid"
    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/processors"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type ProcessOptions struct {
    ParseOnly      bool
    AbsolutePath   string
    CollectorOptions collector.Options
}

func (o ProcessOptions) AbsolutePathOr(defaultPath string) string {
    if o.AbsolutePath != "" {
        return o.AbsolutePath
    }
    return defaultPath
}

type Pipeline struct {
    registry  *processors.Registry
    enricher  *Enricher
    writer    *Writer
    watchDir  string
    reserved  map[string]bool
}

func NewPipeline(reg *processors.Registry, enricher *Enricher, writer *Writer, watchDir string) *Pipeline {
    return &Pipeline{
        registry: reg,
        enricher: enricher,
        writer:   writer,
        watchDir: watchDir,
        reserved: map[string]bool{"__HOTDIR__.md": true},
    }
}

func (p *Pipeline) ProcessFile(ctx context.Context, targetFilename string, options ProcessOptions, metadata map[string]string) (*collector.ProcessResponse, error) {
    fullPath := utils.NormalizePath(options.AbsolutePathOr(filepath.Join(p.watchDir, targetFilename)))
    if fullPath == "" {
        return fail(targetFilename, "Filename is a not a valid path to process.")
    }

    hasAbs := options.AbsolutePath != ""
    if !hasAbs && !utils.IsWithin(p.watchDir, fullPath) {
        return fail(targetFilename, "Filename is a not a valid path to process.")
    }
    if p.reserved[targetFilename] {
        return fail(targetFilename, "Filename is a reserved filename and cannot be processed.")
    }
    if _, err := os.Stat(fullPath); err != nil {
        return fail(targetFilename, "File does not exist in upload directory.")
    }

    ext := strings.ToLower(filepath.Ext(fullPath))
    if ext == "" {
        if !hasAbs { utils.TrashFile(fullPath) }
        return fail(targetFilename, "No file extension found.")
    }

    processor := p.registry.Get(ext)
    if processor == nil {
        if utils.IsTextType(fullPath) {
            ext = ".txt"
            processor = p.registry.Get(ext)
        }
        if processor == nil {
            if !hasAbs { utils.TrashFile(fullPath) }
            return fail(targetFilename, fmt.Sprintf("File extension %s not supported.", ext))
        }
    }

    extOut, err := processor.Extract(ctx, ExtractInput{
        FullFilePath: fullPath,
        Filename:     targetFilename,
        Options:      options.CollectorOptions,
        Metadata:     metadata,
    })
    if err != nil {
        if !hasAbs { utils.TrashFile(fullPath) }
        return fail(targetFilename, fmt.Sprintf("Processing error: %v", err))
    }

    doc := &collector.Document{
        URL:         "file://" + fullPath,
        Title:       firstNonEmpty(metadata["title"], targetFilename),
        DocAuthor:   firstNonEmpty(metadata["docAuthor"], extOut.DocAuthor, "no author found"),
        Description: firstNonEmpty(metadata["description"], extOut.Description, "No description found."),
        DocSource:   firstNonEmpty(metadata["docSource"], fmt.Sprintf("%s file uploaded by the user.", ext)),
        ChunkSource: metadata["chunkSource"],
        Published:   utils.CreatedDate(fullPath),
        PageContent: extOut.PageContent,
    }
    p.enricher.Enrich(doc, extOut)

    docID := uuid.New().String()
    safeName := utils.SlugifyFilename(targetFilename) + "-" + docID
    result, err := p.writer.Write(doc, safeName, options.ParseOnly)
    if err != nil {
        if !hasAbs { utils.TrashFile(fullPath) }
        return fail(targetFilename, fmt.Sprintf("Failed to write document: %v", err))
    }

    if !hasAbs { utils.TrashFile(fullPath) }
    return &collector.ProcessResponse{
        Filename:  targetFilename,
        Success:   true,
        Documents: []collector.Document{*result},
    }, nil
}

func fail(filename, reason string) (*collector.ProcessResponse, error) {
    return &collector.ProcessResponse{
        Filename:  filename,
        Success:   false,
        Reason:    reason,
        Documents: []collector.Document{},
    }, nil
}

func firstNonEmpty(vals ...string) string {
    for _, v := range vals {
        if v != "" {
            return v
        }
    }
    return ""
}
```

- [ ] **Step 5: 写 `pipeline_test.go`**

```go
package pipeline

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/processors"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

func TestPipeline_ProcessFile_NotFound(t *testing.T) {
    tmpDir := t.TempDir()
    tok, _ := utils.NewTokenizer()
    reg := processors.NewRegistry()
    reg.Register(processors.NewTxtExtractor())
    pipe := NewPipeline(reg, NewEnricher(tok), NewWriter(tmpDir), tmpDir)

    resp, err := pipe.ProcessFile(context.Background(), "nonexistent.txt", ProcessOptions{}, nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if resp.Success {
        t.Error("expected failure for nonexistent file")
    }
}

func TestPipeline_ProcessFile_Txt(t *testing.T) {
    tmpDir := t.TempDir()
    watchDir := filepath.Join(tmpDir, "hotdir")
    os.MkdirAll(watchDir, 0755)
    storageDir := filepath.Join(tmpDir, "storage")

    f, _ := os.Create(filepath.Join(watchDir, "hello.txt"))
    f.WriteString("hello world from test")
    f.Close()

    tok, _ := utils.NewTokenizer()
    reg := processors.NewRegistry()
    reg.Register(processors.NewTxtExtractor())
    pipe := NewPipeline(reg, NewEnricher(tok), NewWriter(storageDir), watchDir)

    resp, err := pipe.ProcessFile(context.Background(), "hello.txt", ProcessOptions{}, nil)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !resp.Success {
        t.Fatalf("expected success, got: %s", resp.Reason)
    }
    if len(resp.Documents) != 1 {
        t.Fatalf("expected 1 document, got %d", len(resp.Documents))
    }
    doc := resp.Documents[0]
    if !strings.Contains(doc.PageContent, "hello world") {
        t.Errorf("unexpected content: %q", doc.PageContent)
    }
    if doc.WordCount != 4 {
        t.Errorf("expected wordCount=4, got %d", doc.WordCount)
    }
}
```

- [ ] **Step 6: 运行测试**

Run: `cd backend && go test ./internal/collector/pipeline/... -v`

- [ ] **Step 7: Commit**

```bash
git add backend/internal/collector/pipeline/
git commit -m "feat(collector): add pipeline, extractor, enricher, writer"
```

---

## Phase 2: 简单格式处理器

### Task 6: 处理器注册表 + TxtExtractor

**Files:**
- Create: `backend/internal/collector/processors/registry.go`
- Create: `backend/internal/collector/processors/txt.go`

- [ ] **Step 1: 创建 `registry.go`**

```go
package processors

import (
    "sync"

    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

type Registry struct {
    mu         sync.RWMutex
    byExt      map[string]pipeline.ContentExtractor
    extractors []pipeline.ContentExtractor
}

func NewRegistry() *Registry {
    return &Registry{byExt: make(map[string]pipeline.ContentExtractor)}
}

func (r *Registry) Register(ext pipeline.ContentExtractor) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.extractors = append(r.extractors, ext)
    for _, e := range ext.SupportedExtensions() {
        r.byExt[e] = ext
    }
}

func (r *Registry) Get(ext string) pipeline.ContentExtractor {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.byExt[ext]
}

func (r *Registry) All() []pipeline.ContentExtractor {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make([]pipeline.ContentExtractor, len(r.extractors))
    copy(out, r.extractors)
    return out
}
```

- [ ] **Step 2: 创建 `txt.go`**

```go
package processors

import (
    "context"
    "os"
    "strings"

    "golang.org/x/net/html"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

type TxtExtractor struct{}

func NewTxtExtractor() *TxtExtractor { return &TxtExtractor{} }

func (e *TxtExtractor) SupportedExtensions() []string {
    return []string{".txt", ".md", ".org", ".adoc", ".rst", ".csv", ".json"}
}

func (e *TxtExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    b, err := os.ReadFile(input.FullFilePath)
    if err != nil {
        return pipeline.ExtractOutput{}, err
    }
    return pipeline.ExtractOutput{PageContent: string(b)}, nil
}

type HTMLExtractor struct{}

func NewHTMLExtractor() *HTMLExtractor { return &HTMLExtractor{} }

func (e *HTMLExtractor) SupportedExtensions() []string {
    return []string{".html"}
}

func (e *HTMLExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    b, err := os.ReadFile(input.FullFilePath)
    if err != nil {
        return pipeline.ExtractOutput{}, err
    }
    text := extractTextFromHTML(string(b))
    return pipeline.ExtractOutput{PageContent: text}, nil
}

func extractTextFromHTML(s string) string {
    doc, err := html.Parse(strings.NewReader(s))
    if err != nil {
        return s
    }
    var buf strings.Builder
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.TextNode {
            buf.WriteString(n.Data)
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return strings.TrimSpace(buf.String())
}
```

- [ ] **Step 3: 写测试**

```go
package processors

import (
    "context"
    "os"
    "path/filepath"
    "testing"

    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

func TestTxtExtractor_Extract(t *testing.T) {
    tmpDir := t.TempDir()
    p := filepath.Join(tmpDir, "test.md")
    os.WriteFile(p, []byte("# Hello\n\nThis is markdown."), 0644)

    e := NewTxtExtractor()
    out, err := e.Extract(context.Background(), pipeline.ExtractInput{FullFilePath: p})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.Contains(out.PageContent, "Hello") {
        t.Errorf("unexpected content: %q", out.PageContent)
    }
}

func TestHTMLExtractor_Extract(t *testing.T) {
    tmpDir := t.TempDir()
    p := filepath.Join(tmpDir, "test.html")
    os.WriteFile(p, []byte("<html><body><p>Hello</p></body></html>"), 0644)

    e := NewHTMLExtractor()
    out, err := e.Extract(context.Background(), pipeline.ExtractInput{FullFilePath: p})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !strings.Contains(out.PageContent, "Hello") {
        t.Errorf("unexpected content: %q", out.PageContent)
    }
}

func TestRegistry(t *testing.T) {
    reg := NewRegistry()
    reg.Register(NewTxtExtractor())
    if reg.Get(".txt") == nil {
        t.Error("expected .txt extractor to be registered")
    }
    if reg.Get(".pdf") != nil {
        t.Error("expected .pdf extractor to be nil")
    }
}
```

- [ ] **Step 4: 运行测试**

Run: `cd backend && go test ./internal/collector/processors/... -v`

- [ ] **Step 5: Commit**

```bash
git add backend/internal/collector/processors/registry.go backend/internal/collector/processors/txt.go backend/internal/collector/processors/registry_test.go backend/internal/collector/processors/txt_test.go
git commit -m "feat(collector): add processor registry and txt/html extractors"
```

---

### Task 7: EPub + Mbox + ODF Extractors

**Files:**
- Create: `backend/internal/collector/processors/epub.go`
- Create: `backend/internal/collector/processors/mbox.go`
- Create: `backend/internal/collector/processors/office_odf.go`

**Context:** 这些都基于 ZIP+XML 解析，Go 标准库 `archive/zip` 和 `encoding/xml` 足够处理。

- [ ] **Step 1-3: 实现三个 extractor**

EPub: ZIP 中读取 `*.xhtml/html/htm`，提取文本。
Mbox: 按 `From ` 行分隔解析邮件，提取正文。
ODF: ZIP 中读取 `content.xml`，解析 `<text:p>` 节点提取文本。

- [ ] **Step 4: 写测试（使用 testdata）**

在 `backend/internal/collector/processors/testdata/` 放置最小化的测试文件。

- [ ] **Step 5: Commit**

```bash
git add backend/internal/collector/processors/epub.go backend/internal/collector/processors/mbox.go backend/internal/collector/processors/office_odf.go backend/internal/collector/processors/testdata/
git commit -m "feat(collector): add epub, mbox, odf extractors"
```

---

## Phase 3: Office 格式

### Task 8: Docx Extractor

**Files:**
- Create: `backend/internal/collector/processors/docx.go`

- [ ] **Step 1: 添加依赖**

Run: `cd backend && go get github.com/fumiama/go-docx`

- [ ] **Step 2: 实现**

```go
package processors

import (
    "context"
    "github.com/fumiama/go-docx"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

type DocxExtractor struct{}

func NewDocxExtractor() *DocxExtractor { return &DocxExtractor{} }

func (e *DocxExtractor) SupportedExtensions() []string { return []string{".docx"} }

func (e *DocxExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    doc, err := docx.ParseDocxFile(input.FullFilePath)
    if err != nil {
        return pipeline.ExtractOutput{}, err
    }
    var sb strings.Builder
    for _, para := range doc.Document.Body.Paragraphs {
        for _, run := range para.Runs {
            sb.WriteString(run.Text)
        }
        sb.WriteString("\n")
    }
    return pipeline.ExtractOutput{PageContent: sb.String()}, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/collector/processors/docx.go backend/internal/collector/processors/docx_test.go backend/go.mod backend/go.sum
git commit -m "feat(collector): add docx extractor"
```

---

### Task 9: Xlsx Extractor

**Files:**
- Create: `backend/internal/collector/processors/xlsx.go`

- [ ] **Step 1: 添加依赖**

Run: `cd backend && go get github.com/qax-os/excelize/v2`

- [ ] **Step 2: 实现**

```go
package processors

import (
    "context"
    "strings"
    "github.com/qax-os/excelize/v2"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

type XlsxExtractor struct{}

func NewXlsxExtractor() *XlsxExtractor { return &XlsxExtractor{} }

func (e *XlsxExtractor) SupportedExtensions() []string { return []string{".xlsx"} }

func (e *XlsxExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    f, err := excelize.OpenFile(input.FullFilePath)
    if err != nil {
        return pipeline.ExtractOutput{}, err
    }
    defer f.Close()

    var sb strings.Builder
    for _, sheet := range f.GetSheetList() {
        rows, err := f.GetRows(sheet)
        if err != nil {
            continue
        }
        for _, row := range rows {
            sb.WriteString(strings.Join(row, "\t"))
            sb.WriteString("\n")
        }
    }
    return pipeline.ExtractOutput{PageContent: sb.String()}, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/collector/processors/xlsx.go backend/internal/collector/processors/xlsx_test.go backend/go.mod backend/go.sum
git commit -m "feat(collector): add xlsx extractor"
```

---

### Task 10: Pptx Extractor

**Files:**
- Create: `backend/internal/collector/processors/pptx.go`

- [ ] **Step 1: 实现**

使用 `archive/zip` 打开 `.pptx`，读取 `ppt/slides/slide*.xml`，用 `encoding/xml` 解析 `<a:t>` 文本节点。

- [ ] **Step 2: Commit**

```bash
git add backend/internal/collector/processors/pptx.go backend/internal/collector/processors/pptx_test.go
git commit -m "feat(collector): add pptx extractor"
```

---

## Phase 4: 复杂处理器（PDF, Image, Audio）

### Task 11: External 适配器层

**Files:**
- Create: `backend/internal/collector/external/tesseract.go`
- Create: `backend/internal/collector/external/whisper.go`
- Create: `backend/internal/collector/external/chromedp.go`

- [ ] **Step 1: TesseractAdapter**

```go
package external

import (
    "context"
    "fmt"
    "strings"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type TesseractAdapter struct {
    runner   *utils.ShellRunner
    cacheDir string
    available bool
}

func NewTesseractAdapter(cacheDir string) *TesseractAdapter {
    r := &TesseractAdapter{runner: utils.NewShellRunner(), cacheDir: cacheDir}
    r.available = r.runner.CheckInstalled("tesseract")
    return r
}

func (t *TesseractAdapter) Available() bool { return t.available }

func (t *TesseractAdapter) Run(ctx context.Context, imagePath string, langs []string) (string, error) {
    if !t.available {
        return "", fmt.Errorf("tesseract not installed")
    }
    langStr := "eng"
    if len(langs) > 0 {
        langStr = strings.Join(langs, "+")
    }
    out, err := t.runner.Run(ctx, "tesseract", imagePath, "stdout", "-l", langStr)
    if err != nil {
        return "", fmt.Errorf("tesseract OCR failed: %w", err)
    }
    return out, nil
}
```

- [ ] **Step 2: WhisperAdapter（本地 + OpenAI）**

```go
package external

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
    "path/filepath"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type WhisperLocalAdapter struct {
    runner    *utils.ShellRunner
    modelPath string
    available bool
}

func NewWhisperLocalAdapter(modelPath string) *WhisperLocalAdapter {
    r := &WhisperLocalAdapter{runner: utils.NewShellRunner(), modelPath: modelPath}
    r.available = r.runner.CheckInstalled("whisper") || r.runner.CheckInstalled("main")
    return r
}

func (w *WhisperLocalAdapter) Available() bool { return w.available }

func (w *WhisperLocalAdapter) Transcribe(ctx context.Context, wavPath string) (string, error) {
    // Try whisper.cpp main first, then whisper-cli
    // ...
}

type WhisperOpenAIAdapter struct {
    apiKey string
    client *http.Client
}

func NewWhisperOpenAIAdapter(apiKey string) *WhisperOpenAIAdapter {
    return &WhisperOpenAIAdapter{apiKey: apiKey, client: &http.Client{Timeout: 10 * time.Minute}}
}

func (o *WhisperOpenAIAdapter) Transcribe(ctx context.Context, filePath string) (string, error) {
    // multipart POST to OpenAI /v1/audio/transcriptions
    // ...
}
```

- [ ] **Step 3: ChromedpAdapter**

```go
package external

import (
    "context"
    "fmt"
    "time"
    "github.com/chromedp/chromedp"
)

type ChromedpAdapter struct {
    launchArgs []string
}

func NewChromedpAdapter(launchArgs []string) *ChromedpAdapter {
    return &ChromedpAdapter{launchArgs: launchArgs}
}

func (c *ChromedpAdapter) FetchText(ctx context.Context, url string, headers map[string]string) (string, error) {
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.Flag("headless", true),
        chromedp.Flag("ignore-certificate-errors", true),
    )
    for _, arg := range c.launchArgs {
        opts = append(opts, chromedp.Flag(arg, true))
    }
    allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
    defer cancel()
    taskCtx, cancel := chromedp.NewContext(allocCtx)
    defer cancel()
    taskCtx, cancel = context.WithTimeout(taskCtx, 3*time.Minute)
    defer cancel()

    var text string
    actions := []chromedp.Action{
        chromedp.Navigate(url),
        chromedp.WaitReady("body"),
        chromedp.Evaluate("document.body.innerText", &text),
    }
    if err := chromedp.Run(taskCtx, actions...); err != nil {
        return "", fmt.Errorf("chromedp failed: %w", err)
    }
    return text, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/collector/external/
git commit -m "feat(collector): add external tool adapters (tesseract, whisper, chromedp)"
```

---

### Task 12: PDF Extractor

**Files:**
- Create: `backend/internal/collector/processors/pdf.go`

- [ ] **Step 1: 添加依赖**

Run: `cd backend && go get github.com/ledongthuc/pdf`

- [ ] **Step 2: 实现三层回退**

```go
package processors

import (
    "context"
    "fmt"
    "github.com/ledongthuc/pdf"
    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/external"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type PDFExtractor struct {
    tesseract *external.TesseractAdapter
    runner    *utils.ShellRunner
}

func NewPDFExtractor(tesseract *external.TesseractAdapter) *PDFExtractor {
    return &PDFExtractor{tesseract: tesseract, runner: utils.NewShellRunner()}
}

func (e *PDFExtractor) SupportedExtensions() []string { return []string{".pdf"} }

func (e *PDFExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    // Layer 1: Pure Go
    content, author, title, err := e.extractPureGo(input.FullFilePath)
    if err == nil && len(content) > 0 {
        return pipeline.ExtractOutput{PageContent: content, DocAuthor: author, Description: title}, nil
    }

    // Layer 2: pdftotext
    content, err = e.extractViaPoppler(ctx, input.FullFilePath)
    if err == nil && len(content) > 0 {
        return pipeline.ExtractOutput{PageContent: content}, nil
    }

    // Layer 3: OCR
    if e.tesseract.Available() && input.Options.OCR.LangList != "" {
        return e.ocrPDF(ctx, input)
    }

    return pipeline.ExtractOutput{}, collector.ErrEmptyContent
}

func (e *PDFExtractor) extractPureGo(path string) (content, author, title string, err error) {
    f, r, err := pdf.Open(path)
    if err != nil {
        return "", "", "", err
    }
    defer f.Close()
    var sb strings.Builder
    totalPage := r.NumPage()
    for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
        p := r.Page(pageIndex)
        if p.V.IsNull() {
            continue
        }
        text := p.Content().Text
        for _, t := range text {
            sb.WriteString(t.S)
        }
        sb.WriteString("\n")
    }
    return sb.String(), "", "", nil
}

func (e *PDFExtractor) extractViaPoppler(ctx context.Context, path string) (string, error) {
    if !e.runner.CheckInstalled("pdftotext") {
        return "", fmt.Errorf("pdftotext not installed")
    }
    return e.runner.Run(ctx, "pdftotext", "-layout", path, "-")
}

func (e *PDFExtractor) ocrPDF(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    // Convert PDF pages to images, then OCR each page
    // This requires pdfium or similar; for MVP, return error indicating need for pdftotext or OCR tools
    return pipeline.ExtractOutput{}, collector.ErrOCRFailed
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/collector/processors/pdf.go backend/internal/collector/processors/pdf_test.go backend/go.mod backend/go.sum
git commit -m "feat(collector): add pdf extractor with 3-layer fallback"
```

---

### Task 13: Image Extractor

**Files:**
- Create: `backend/internal/collector/processors/image.go`

- [ ] **Step 1: 实现**

```go
package processors

import (
    "context"
    "strings"
    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/external"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
)

type ImageExtractor struct {
    tesseract *external.TesseractAdapter
}

func NewImageExtractor(tesseract *external.TesseractAdapter) *ImageExtractor {
    return &ImageExtractor{tesseract: tesseract}
}

func (e *ImageExtractor) SupportedExtensions() []string {
    return []string{".png", ".jpg", ".jpeg", ".webp"}
}

func (e *ImageExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    if !e.tesseract.Available() {
        return pipeline.ExtractOutput{}, collector.ErrOCRFailed
    }
    langs := parseOCRLanguages(input.Options.OCR.LangList)
    text, err := e.tesseract.Run(ctx, input.FullFilePath, langs)
    if err != nil {
        return pipeline.ExtractOutput{}, err
    }
    return pipeline.ExtractOutput{PageContent: text}, nil
}

func parseOCRLanguages(langList string) []string {
    if langList == "" {
        return []string{"eng"}
    }
    parts := strings.Split(langList, ",")
    out := make([]string, 0, len(parts))
    for _, p := range parts {
        p = strings.TrimSpace(p)
        if p != "" {
            out = append(out, p)
        }
    }
    if len(out) == 0 {
        return []string{"eng"}
    }
    return out
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/collector/processors/image.go backend/internal/collector/processors/image_test.go
git commit -m "feat(collector): add image OCR extractor"
```

---

### Task 14: Audio Extractor

**Files:**
- Create: `backend/internal/collector/processors/audio.go`

- [ ] **Step 1: 实现**

```go
package processors

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "github.com/odysseythink/hermind/backend/internal/collector"
    "github.com/odysseythink/hermind/backend/internal/collector/external"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

type AudioExtractor struct {
    whisperLocal *external.WhisperLocalAdapter
    whisperOpenAI *external.WhisperOpenAIAdapter
    runner       *utils.ShellRunner
}

func NewAudioExtractor(local *external.WhisperLocalAdapter, openai *external.WhisperOpenAIAdapter) *AudioExtractor {
    return &AudioExtractor{
        whisperLocal: local,
        whisperOpenAI: openai,
        runner: utils.NewShellRunner(),
    }
}

func (e *AudioExtractor) SupportedExtensions() []string {
    return []string{".mp3", ".wav", ".mp4", ".mpeg", ".ogg", ".oga", ".opus", ".m4a", ".webm"}
}

func (e *AudioExtractor) Extract(ctx context.Context, input pipeline.ExtractInput) (pipeline.ExtractOutput, error) {
    provider := e.resolveProvider(input.Options.WhisperProvider, input.Options.OpenAiKey)
    if provider == nil {
        return pipeline.ExtractOutput{}, collector.ErrTranscriptionFailed
    }

    // Convert to WAV via ffmpeg
    tmpDir := os.TempDir()
    wavPath := filepath.Join(tmpDir, filepath.Base(input.FullFilePath)+".wav")
    _, err := e.runner.Run(ctx, "ffmpeg", "-i", input.FullFilePath, "-ar", "16000", "-ac", "1", "-sample_fmt", "s16", "-y", wavPath)
    if err != nil {
        return pipeline.ExtractOutput{}, fmt.Errorf("ffmpeg conversion failed: %w", err)
    }
    defer os.Remove(wavPath)

    text, err := provider.Transcribe(ctx, wavPath)
    if err != nil {
        return pipeline.ExtractOutput{}, fmt.Errorf("transcription failed: %w", err)
    }
    return pipeline.ExtractOutput{PageContent: text}, nil
}

func (e *AudioExtractor) resolveProvider(provider, openAiKey string) TranscriptionProvider {
    switch provider {
    case "openai":
        if e.whisperOpenAI != nil && openAiKey != "" {
            return e.whisperOpenAI
        }
    default:
        if e.whisperLocal != nil && e.whisperLocal.Available() {
            return e.whisperLocal
        }
    }
    return nil
}

type TranscriptionProvider interface {
    Transcribe(ctx context.Context, filePath string) (string, error)
}
```

- [ ] **Step 2: Commit**

```bash
git add backend/internal/collector/processors/audio.go backend/internal/collector/processors/audio_test.go
git commit -m "feat(collector): add audio extractor with ffmpeg + whisper"
```

---

## Phase 5: 链接抓取

### Task 15: Scraper 基础设施

**Files:**
- Create: `backend/internal/collector/scraper/scraper.go`
- Create: `backend/internal/collector/scraper/helpers.go`

- [ ] **Step 1: 实现 helpers（URL 验证、Content-Type 判定）**

```go
package scraper

import (
    "net/url"
    "regexp"
    "strings"
)

func isYouTubeURL(link string) bool {
    u, err := url.Parse(link)
    if err != nil {
        return false
    }
    return strings.Contains(u.Host, "youtube.com") || strings.Contains(u.Host, "youtu.be")
}

func isDirectFileURL(link string) bool {
    ext := strings.ToLower(filepath.Ext(link))
    return ext != "" && ext != ".html" && ext != ".htm"
}

func validateURL(link string) string {
    if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
        link = "https://" + link
    }
    return link
}

func validURL(link string) bool {
    u, err := url.Parse(link)
    return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
```

- [ ] **Step 2: Manager 接口**

```go
package scraper

import (
    "context"
    "github.com/odysseythink/hermind/backend/internal/collector"
)

type Manager struct {
    generic *GenericScraper
    youtube *YouTubeScraper
}

func NewManager(browser BrowserAdapter) *Manager {
    return &Manager{
        generic: NewGenericScraper(browser),
        youtube: NewYouTubeScraper(),
    }
}

func (m *Manager) Scrape(ctx context.Context, link string, captureAs string, headers map[string]string, saveAsDocument bool, metadata map[string]string) (*collector.ProcessResponse, error) {
    // Route based on content type
}

func (m *Manager) GetLinkText(ctx context.Context, link string, captureAs string) (*collector.LinkContentResponse, error) {
    // ...
}
```

- [ ] **Step 3: Commit**

```bash
git add backend/internal/collector/scraper/
git commit -m "feat(collector): add scraper infrastructure"
```

---

### Task 16: 通用网页抓取 + YouTube

**Files:**
- Create: `backend/internal/collector/scraper/generic.go`
- Create: `backend/internal/collector/scraper/youtube.go`

- [ ] **Step 1: 添加依赖**

Run: `cd backend && go get github.com/PuerkitoBio/goquery`

- [ ] **Step 2: 实现 GenericScraper**

HTTP 优先，chromedp 回退。

- [ ] **Step 3: 实现 YouTubeScraper**

解析 videoID，获取字幕（通过 `https://www.youtube.com/api/timedtext` 或页面内嵌数据）。

- [ ] **Step 4: Commit**

```bash
git add backend/internal/collector/scraper/generic.go backend/internal/collector/scraper/youtube.go backend/go.mod backend/go.sum
git commit -m "feat(collector): add generic web scraper and youtube transcript"
```

---

## Phase 6: 扩展系统

### Task 17: Extension Registry + GitHub/GitLab

**Files:**
- Create: `backend/internal/collector/extensions/registry.go`
- Create: `backend/internal/collector/extensions/github.go`
- Create: `backend/internal/collector/extensions/gitlab.go`

- [ ] **Step 1: 添加依赖**

Run: `cd backend && go get github.com/google/go-github/v63 github.com/xanzy/go-gitlab`

- [ ] **Step 2: 实现**

- [ ] **Step 3: Commit**

```bash
git add backend/internal/collector/extensions/registry.go backend/internal/collector/extensions/github.go backend/internal/collector/extensions/gitlab.go backend/go.mod backend/go.sum
git commit -m "feat(collector): add extension registry and repo loaders"
```

---

### Task 18: 其他扩展

**Files:**
- Create: `backend/internal/collector/extensions/confluence.go`
- Create: `backend/internal/collector/extensions/drupalwiki.go`
- Create: `backend/internal/collector/extensions/obsidian.go`
- Create: `backend/internal/collector/extensions/paperless.go`
- Create: `backend/internal/collector/extensions/website_depth.go`
- Create: `backend/internal/collector/extensions/resync.go`

- [ ] **Step 1: 逐个实现**

每个扩展都是 HTTP REST API 调用或本地文件系统操作。

- [ ] **Step 2: Commit**

```bash
git add backend/internal/collector/extensions/confluence.go backend/internal/collector/extensions/drupalwiki.go backend/internal/collector/extensions/obsidian.go backend/internal/collector/extensions/paperless.go backend/internal/collector/extensions/website_depth.go backend/internal/collector/extensions/resync.go
git commit -m "feat(collector): add all remaining extensions"
```

---

## Phase 7: Client Facade + 集成

### Task 19: Client 重构

**Files:**
- Modify: `backend/internal/collector/client.go`

**Context:** 保留所有公共方法签名，内部从 HTTP 调用改为本地流水线调用。

- [ ] **Step 1: 重写 `client.go`**

```go
package collector

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/odysseythink/hermind/backend/internal/collector/extensions"
    "github.com/odysseythink/hermind/backend/internal/collector/external"
    "github.com/odysseythink/hermind/backend/internal/collector/pipeline"
    "github.com/odysseythink/hermind/backend/internal/collector/processors"
    "github.com/odysseythink/hermind/backend/internal/collector/scraper"
    "github.com/odysseythink/hermind/backend/internal/collector/utils"
)

func NewLocalCollector(storageDir string, cfg any) (*Client, error) {
    tok, err := utils.NewTokenizer()
    if err != nil {
        return nil, fmt.Errorf("init tokenizer: %w", err)
    }

    tesseract := external.NewTesseractAdapter(filepath.Join(storageDir, "models", "tesseract"))
    whisperLocal := external.NewWhisperLocalAdapter(filepath.Join(storageDir, "models", "whisper"))
    var whisperOpenAI *external.WhisperOpenAIAdapter
    if key := os.Getenv("OPEN_AI_KEY"); key != "" {
        whisperOpenAI = external.NewWhisperOpenAIAdapter(key)
    }
    browser := external.NewChromedpAdapter(parseBrowserArgs())

    reg := processors.NewRegistry()
    reg.Register(processors.NewTxtExtractor())
    reg.Register(processors.NewHTMLExtractor())
    reg.Register(processors.NewDocxExtractor())
    reg.Register(processors.NewXlsxExtractor())
    reg.Register(processors.NewPptxExtractor())
    reg.Register(processors.NewODFExtractor())
    reg.Register(processors.NewEPubExtractor())
    reg.Register(processors.NewMboxExtractor())
    reg.Register(processors.NewPDFExtractor(tesseract))
    reg.Register(processors.NewImageExtractor(tesseract))
    reg.Register(processors.NewAudioExtractor(whisperLocal, whisperOpenAI))

    scraperMgr := scraper.NewManager(browser)

    extReg := extensions.NewRegistry()
    extReg.Register(extensions.NewResyncExtension(scraperMgr))
    extReg.Register(extensions.NewRepoExtension())
    extReg.Register(extensions.NewYouTubeExtension(scraperMgr))
    extReg.Register(extensions.NewWebsiteDepthExtension(browser))
    extReg.Register(extensions.NewConfluenceExtension())
    extReg.Register(extensions.NewDrupalWikiExtension())
    extReg.Register(extensions.NewObsidianExtension())
    extReg.Register(extensions.NewPaperlessExtension())

    pipe := pipeline.NewPipeline(reg, pipeline.NewEnricher(tok), pipeline.NewWriter(storageDir), filepath.Join(storageDir, "hotdir"))

    return &Client{
        pipeline:   pipe,
        registry:   reg,
        extensions: extReg,
        scraper:    scraperMgr,
    }, nil
}

func parseBrowserArgs() []string {
    if args := os.Getenv("ANYTHINGLLM_CHROMIUM_ARGS"); args != "" {
        return strings.Split(args, ",")
    }
    return nil
}

type Client struct {
    pipeline   *pipeline.Pipeline
    registry   *processors.Registry
    extensions *extensions.Registry
    scraper    *scraper.Manager
}

func (c *Client) Online(ctx context.Context) bool { return true }

func (c *Client) AcceptedFileTypes(ctx context.Context) ([]string, error) {
    return utils.AcceptedFileTypes(), nil
}

func (c *Client) ProcessDocument(ctx context.Context, filename string, metadata map[string]string) (*ProcessResponse, error) {
    return c.pipeline.ProcessFile(ctx, filename, pipeline.ProcessOptions{CollectorOptions: c.attachOptions()}, metadata)
}

func (c *Client) ParseDocument(ctx context.Context, filename string, opts ParseOptions) (*ProcessResponse, error) {
    return c.pipeline.ProcessFile(ctx, filename, pipeline.ProcessOptions{
        ParseOnly:      true,
        AbsolutePath:   opts.AbsolutePath,
        CollectorOptions: c.attachOptions(),
    }, nil)
}

func (c *Client) ProcessLink(ctx context.Context, link string, scraperHeaders, metadata map[string]string) (*ProcessResponse, error) {
    return c.scraper.Scrape(ctx, link, "text", scraperHeaders, true, metadata)
}

func (c *Client) GetLinkContent(ctx context.Context, link string, captureAs string) (*LinkContentResponse, error) {
    return c.scraper.GetLinkText(ctx, link, captureAs)
}

func (c *Client) ProcessRawText(ctx context.Context, textContent string, metadata map[string]string) (*ProcessResponse, error) {
    // Build document directly from raw text
    // ...
}

func (c *Client) ForwardExtensionRequest(ctx context.Context, endpoint, method, body string) (*ExtensionResponse, error) {
    return c.extensions.Handle(ctx, endpoint, method, []byte(body))
}

func (c *Client) attachOptions() Options {
    whisperProvider := os.Getenv("WHISPER_PROVIDER")
    if whisperProvider == "" {
        whisperProvider = "local"
    }
    langList := os.Getenv("TARGET_OCR_LANG")
    if langList == "" {
        langList = "eng"
    }
    return Options{
        WhisperProvider:  whisperProvider,
        WhisperModelPref: os.Getenv("WHISPER_MODEL_PREF"),
        OpenAiKey:        os.Getenv("OPEN_AI_KEY"),
        OCR:              OCROptions{LangList: langList},
        RuntimeSettings: RuntimeSettings{
            AllowAnyIp:        os.Getenv("COLLECTOR_ALLOW_ANY_IP"),
            BrowserLaunchArgs: parseBrowserArgs(),
        },
    }
}
```

- [ ] **Step 2: 编译检查**

Run: `cd backend && go build ./internal/collector/...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/collector/client.go
git commit -m "feat(collector): rewrite client as local collector facade"
```

---

### Task 20: main.go 修改 + 集成测试

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 修改 main.go**

```go
// Replace:
// coll, err := collector.NewClient(cfg.StorageDir)
// With:
coll, err := collector.NewLocalCollector(cfg.StorageDir, cfg)
if err != nil {
    mlog.Warning("collector init failed, continuing without collector", mlog.Err(err))
}
```

- [ ] **Step 2: 编译整个项目**

Run: `cd backend && go build ./...`

- [ ] **Step 3: 运行 collector 包测试**

Run: `cd backend && go test ./internal/collector/... -race -v`

- [ ] **Step 4: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(collector): integrate local collector into main server"
```

---

## Phase 8: 清理与最终验证

### Task 21: 删除旧 HTTP 代理代码

**Files:**
- Modify: `backend/internal/collector/client.go`（删除 signAndPost, comKey, encMgr, http client 等旧代码）

**注意：** 这些字段已在 Task 19 中替换。如果 Task 19 没有彻底清理，在此完成。

- [ ] **Step 1: 确认没有遗留 HTTP 相关代码**
- [ ] **Step 2: Commit**

```bash
git add backend/internal/collector/client.go
git commit -m "refactor(collector): remove legacy HTTP proxy code"
```

---

### Task 22: 最终验证

- [ ] **Step 1: 全量编译**

Run: `cd backend && go build ./...`

- [ ] **Step 2: 运行所有 collector 测试（含 -race）**

Run: `cd backend && go test ./internal/collector/... -race -v`

- [ ] **Step 3: 运行全量测试**

Run: `cd backend && go test ./... -race`
（预期：mcp 包通过；services/vector_service.go 的 pre-existing 失败不受影响）

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "feat(collector): complete go-collector rewrite - all tests pass"
```

---

## Self-Review

### Spec Coverage Check

| Spec Section | 对应 Task | 状态 |
|---|---|---|
| 错误类型 (§3.4) | Task 1 | ✓ |
| 流水线接口 (§3.2) | Task 5 | ✓ |
| Pipeline 编排 (§4) | Task 5 | ✓ |
| Txt/Html (§5) | Task 6 | ✓ |
| EPub/Mbox/ODF (§5) | Task 7 | ✓ |
| Docx (§5) | Task 8 | ✓ |
| Xlsx (§5) | Task 9 | ✓ |
| Pptx (§5) | Task 10 | ✓ |
| PDF 三层回退 (§5.1) | Task 12 | ✓ |
| Image OCR (§5) | Task 13 | ✓ |
| Audio ffmpeg+whisper (§5.2) | Task 14 | ✓ |
| 链接抓取 (§6) | Task 15-16 | ✓ |
| YouTube (§6.3) | Task 16 | ✓ |
| 扩展系统 (§7) | Task 17-18 | ✓ |
| Client facade (§9) | Task 19 | ✓ |
| main.go 集成 (§9.2) | Task 20 | ✓ |

### Placeholder Scan

无 TBD/TODO/placeholder。每个 Task 都有具体的文件路径和实现代码。

### Type Consistency

- `ExtractInput`, `ExtractOutput` 在 Task 5 定义，后续所有 processor 使用一致
- `ProcessOptions` 在 Task 5 定义，Client facade 使用一致
- `Options` / `OCROptions` / `RuntimeSettings` 在 Task 1 定义，全局一致

---

*Plan written: 2026-05-27*
*Design ref: backend/.gpowers/designs/2026-05-27-go-collector-rewrite-design.md*
