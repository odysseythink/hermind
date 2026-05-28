# Go Collector 重构设计

## 背景

现有的 `collector/` 是一个 Node.js 独立服务（7857 行 JavaScript），监听 8888 端口，负责文档收集、处理、转换。`backend/internal/collector/client.go` 目前只是一个 HTTP 代理客户端，将所有请求转发给 Node collector。

本设计目标：**用 Go 100% 原生实现 collector 的所有功能**，作为 `backend/internal/collector/` 的内部包，彻底消除对 Node.js 的依赖。

---

## 1. 设计原则

1. **向后兼容**：`collector.Client` 的公共方法签名保持不变，现有调用方（`DocumentService`、`SystemHandler`、`handlers`）无需修改
2. **接口驱动**：所有处理逻辑通过接口抽象，便于测试和扩展
3. **统一流水线**：文件处理遵循 `Extract → Enrich → Write` 的统一模式，消除 20+ 个 converter 中的重复代码
4. **外部依赖务实**：Go 生态成熟的领域用纯 Go，薄弱领域调用外部 CLI

---

## 2. 目录结构

```
backend/internal/collector/
├── client.go              # Facade，保留公共 API，改本地实现
├── types.go               # DTO（ProcessResponse, Document 等）
├── options.go             # 处理选项
├── errors.go              # 错误类型定义
│
├── pipeline/              # 核心流水线
│   ├── pipeline.go        # Extract → Enrich → Write 编排
│   ├── extractor.go       # ContentExtractor 接口
│   ├── enricher.go        # DocumentEnricher（元数据、token 计数）
│   └── writer.go          # DocumentWriter（JSON 写入 storage）
│
├── processors/            # 文件格式处理器
│   ├── registry.go        # 扩展名 → Processor 映射
│   ├── txt.go             # .txt, .md, .org, .adoc, .rst, .csv, .json, .html
│   ├── pdf.go             # .pdf（纯 Go + pdftotext fallback + OCR fallback）
│   ├── docx.go            # .docx
│   ├── xlsx.go            # .xlsx
│   ├── pptx.go            # .pptx
│   ├── office_odf.go      # .odt, .odp
│   ├── epub.go            # .epub
│   ├── mbox.go            # .mbox
│   ├── image.go           # .png, .jpg, .jpeg, .webp（OCR）
│   └── audio.go           # .mp3, .wav, .mp4, .mpeg, .ogg, .oga, .opus, .m4a, .webm
│
├── scraper/               # 链接抓取
│   ├── scraper.go         # LinkScraper 接口
│   ├── generic.go         # 通用网页抓取（goquery / chromedp）
│   ├── youtube.go         # YouTube 字幕提取
│   └── helpers.go         # URL 验证、Content-Type 判定
│
├── extensions/            # 数据源扩展
│   ├── registry.go        # Extension 注册表
│   ├── github.go          # GitHub RepoLoader
│   ├── gitlab.go          # GitLab RepoLoader
│   ├── confluence.go      # Confluence
│   ├── drupalwiki.go      # DrupalWiki
│   ├── obsidian.go        # Obsidian Vault
│   ├── paperless.go       # PaperlessNgx
│   ├── website_depth.go   # 网站深度抓取
│   └── resync.go          # Resync 方法集
│
├── utils/                 # 支撑工具
│   ├── tokenizer.go       # tiktoken-go 封装
│   ├── files.go           # 文件操作（writeToServerDocuments 等）
│   ├── mime.go            # MIME 检测
│   ├── text.go            # 文本检测（isTextType）
│   └── shell.go           # 外部命令调用封装
│
└── external/              # 外部工具适配器
    ├── tesseract.go       # OCR CLI 封装
    ├── whisper.go         # Whisper CLI / OpenAI API 封装
    └── chromedp.go        # 浏览器抓取适配
```

---

## 3. 核心类型与接口

### 3.1 保留的现有类型

```go
type Options struct {
    WhisperProvider  string          `json:"whisperProvider"`
    WhisperModelPref string          `json:"whisperModelPref,omitempty"`
    OpenAiKey        string          `json:"openAiKey,omitempty"`
    OCR              OCROptions      `json:"ocr"`
    RuntimeSettings  RuntimeSettings `json:"runtimeSettings"`
}

type ProcessResponse struct {
    Filename  string     `json:"filename"`
    Success   bool       `json:"success"`
    Reason    string     `json:"reason"`
    Documents []Document `json:"documents"`
}

type Document struct {
    Location           string `json:"location"`
    Name               string `json:"name"`
    URL                string `json:"url"`
    Title              string `json:"title"`
    DocAuthor          string `json:"docAuthor"`
    Description        string `json:"description"`
    DocSource          string `json:"docSource"`
    ChunkSource        string `json:"chunkSource"`
    Published          string `json:"published"`
    WordCount          int    `json:"wordCount"`
    TokenCountEstimate int    `json:"token_count_estimate"`
}
```

### 3.2 流水线接口

```go
package pipeline

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

type DocumentWriter interface {
    Write(doc *collector.Document, filename string, opts WriteOptions) (*collector.Document, error)
}

type WriteOptions struct {
    ParseOnly    bool
    Destination  string
}
```

### 3.3 Client 公共 API（向后兼容）

```go
type Client struct {
    pipeline   *pipeline.Pipeline
    registry   *processors.Registry
    extensions *extensions.Registry
    scraper    *scraper.Manager
    options    collector.Options
}

func (c *Client) ProcessDocument(ctx context.Context, filename string, metadata map[string]string) (*ProcessResponse, error)
func (c *Client) ParseDocument(ctx context.Context, filename string, opts ParseOptions) (*ProcessResponse, error)
func (c *Client) ProcessLink(ctx context.Context, link string, scraperHeaders, metadata map[string]string) (*ProcessResponse, error)
func (c *Client) GetLinkContent(ctx context.Context, link string, captureAs string) (*LinkContentResponse, error)
func (c *Client) ProcessRawText(ctx context.Context, textContent string, metadata map[string]string) (*ProcessResponse, error)
func (c *Client) AcceptedFileTypes(ctx context.Context) ([]string, error)
func (c *Client) ForwardExtensionRequest(ctx context.Context, endpoint, method, body string) (*ExtensionResponse, error)
func (c *Client) Online(ctx context.Context) bool // 总是返回 true
```

### 3.4 错误类型

```go
var (
    ErrUnsupportedFormat    = errors.New("unsupported file format")
    ErrFileNotFound         = errors.New("file not found")
    ErrInvalidPath          = errors.New("invalid file path")
    ErrEmptyContent         = errors.New("no text content found")
    ErrProcessingTimeout    = errors.New("processing timeout")
    ErrOCRFailed            = errors.New("OCR processing failed")
    ErrTranscriptionFailed  = errors.New("audio transcription failed")
    ErrBrowserLaunchFailed  = errors.New("browser launch failed")
)
```

---

## 4. 文件处理流水线

### 4.1 Pipeline 编排

```go
func (p *Pipeline) ProcessFile(ctx context.Context, targetFilename string, options ProcessOptions, metadata map[string]string) (*ProcessResponse, error) {
    // 1. 路径解析与安全检查
    fullPath := normalizePath(options.AbsolutePathOr(filepath.Join(watchDir, targetFilename)))

    if !options.HasAbsolutePath && !isWithin(watchDir, fullPath) {
        return fail("Filename is a not a valid path to process.")
    }
    if isReservedFile(targetFilename) {
        return fail("Filename is a reserved filename and cannot be processed.")
    }
    if _, err := os.Stat(fullPath); err != nil {
        return fail("File does not exist in upload directory.")
    }

    // 2. 扩展名解析
    ext := strings.ToLower(filepath.Ext(fullPath))
    if ext == "" {
        return fail("No file extension found.")
    }

    // 3. 尝试文本回退
    processor := p.registry.Get(ext)
    if processor == nil {
        if isTextType(fullPath) {
            ext = ".txt"
            processor = p.registry.Get(ext)
        } else {
            if !options.HasAbsolutePath { trashFile(fullPath) }
            return fail(fmt.Sprintf("File extension %s not supported.", ext))
        }
    }

    // 4. 提取内容
    extOut, err := processor.Extract(ctx, ExtractInput{
        FullFilePath: fullPath,
        Filename:     targetFilename,
        Options:      options.CollectorOptions,
        Metadata:     metadata,
    })
    if err != nil {
        if !options.HasAbsolutePath { trashFile(fullPath) }
        return fail(fmt.Sprintf("Processing error: %v", err))
    }

    // 5. 构建 Document
    doc := &collector.Document{
        URL:         "file://" + fullPath,
        Title:       firstNonEmpty(metadata["title"], targetFilename),
        DocAuthor:   firstNonEmpty(metadata["docAuthor"], extOut.DocAuthor, "no author found"),
        Description: firstNonEmpty(metadata["description"], extOut.Description, "No description found."),
        DocSource:   firstNonEmpty(metadata["docSource"], fmt.Sprintf("%s file uploaded by the user.", ext)),
        ChunkSource: metadata["chunkSource"],
        Published:   fileCreatedDate(fullPath),
        PageContent: extOut.PageContent,
    }
    doc.WordCount = len(strings.Fields(doc.PageContent))

    // 6. 丰富（token 计数）
    doc.TokenCountEstimate = p.tokenizer.Count(doc.PageContent)

    // 7. 写入存储
    docID := uuid.New().String()
    safeName := slugify(targetFilename) + "-" + docID
    result, err := p.writer.Write(doc, safeName, WriteOptions{
        ParseOnly: options.ParseOnly,
    })
    if err != nil {
        if !options.HasAbsolutePath { trashFile(fullPath) }
        return fail(fmt.Sprintf("Failed to write document: %v", err))
    }

    // 8. 清理
    if !options.HasAbsolutePath { trashFile(fullPath) }

    return &ProcessResponse{
        Filename:  targetFilename,
        Success:   true,
        Documents: []collector.Document{*result},
    }, nil
}
```

---

## 5. 格式处理器矩阵

| 扩展名 | 处理器 | 实现策略 | 外部依赖 |
|---|---|---|---|
| `.txt`, `.md`, `.org`, `.adoc`, `.rst`, `.csv`, `.json` | `TxtExtractor` | 标准库 `os.ReadFile` | 无 |
| `.html` | `TxtExtractor` | `golang.org/x/net/html` 提取 body text | 无 |
| `.pdf` | `PDFExtractor` | 先用 `github.com/ledongthuc/pdf` 纯 Go 提取；失败则调用 `pdftotext`；再失败则 OCR | `pdftotext` (optional), `tesseract` (optional) |
| `.docx` | `DocxExtractor` | `github.com/fumiama/go-docx` 读取 paragraphs | 无 |
| `.xlsx` | `XlsxExtractor` | `github.com/qax-os/excelize` 读取 cells | 无 |
| `.pptx` | `PptxExtractor` | `archive/zip` + `encoding/xml` 解析 `ppt/slides/*.xml` | 无 |
| `.odt`, `.odp` | `ODFExtractor` | `archive/zip` + XML 解析 `content.xml` | 无 |
| `.epub` | `EPubExtractor` | `archive/zip` 读取 `*.xhtml/html/htm` | 无 |
| `.mbox` | `MboxExtractor` | 自定义按 `From ` 分隔符解析 | 无 |
| `.png`, `.jpg`, `.jpeg`, `.webp` | `ImageExtractor` | 调用 `tesseract` CLI | `tesseract` + 语言包 |
| `.mp3`, `.wav`, `.mp4`, `.mpeg`, `.ogg`, `.oga`, `.opus`, `.m4a`, `.webm` | `AudioExtractor` | `ffmpeg` 转 WAV → `whisper.cpp` 或 OpenAI API | `ffmpeg`, `whisper` / OpenAI key |

### 5.1 PDF 三层回退

```go
func (e *PDFExtractor) Extract(ctx context.Context, input ExtractInput) (ExtractOutput, error) {
    // Layer 1: 纯 Go (ledongthuc/pdf)
    content, author, title, err := e.extractPureGo(input.FullFilePath)
    if err == nil && len(content) > 0 {
        return ExtractOutput{PageContent: content, DocAuthor: author, Description: title}, nil
    }

    // Layer 2: pdftotext CLI
    content, err = e.extractViaPoppler(ctx, input.FullFilePath)
    if err == nil && len(content) > 0 {
        return ExtractOutput{PageContent: content}, nil
    }

    // Layer 3: OCR（扫描版 PDF）
    if input.Options.OCR.LangList != "" {
        return e.ocrPDF(ctx, input)
    }

    return ExtractOutput{}, ErrEmptyContent
}
```

### 5.2 音频处理流水线

```go
func (e *AudioExtractor) Extract(ctx context.Context, input ExtractInput) (ExtractOutput, error) {
    provider := e.resolveProvider(input.Options.WhisperProvider)

    // ffmpeg 转码为 16kHz WAV
    wavPath, err := e.ffmpeg.ConvertToWav(ctx, input.FullFilePath)
    if err != nil {
        return ExtractOutput{}, err
    }
    defer os.Remove(wavPath)

    text, err := provider.Transcribe(ctx, wavPath)
    if err != nil {
        return ExtractOutput{}, fmt.Errorf("%w: %v", ErrTranscriptionFailed, err)
    }
    return ExtractOutput{PageContent: text}, nil
}
```

---

## 6. 链接抓取系统

### 6.1 内容类型判定

```go
func determineContentType(link string) ContentType {
    if isYouTubeURL(link) {
        return ContentType{Kind: "youtube", ProcessVia: "youtube"}
    }
    if isDirectFileURL(link) {
        return ContentType{Kind: "file", ProcessVia: "file"}
    }
    return ContentType{Kind: "web", ProcessVia: "web"}
}
```

### 6.2 抓取回退策略

1. **HTTP 抓取**（第一优先级）：`net/http` + `goquery` 提取文本/HTML
2. **浏览器抓取**（第二优先级）：`chromedp` 渲染 JS 页面
3. **失败返回错误**

### 6.3 YouTube 字幕提取

解析 videoID → 尝试获取自动字幕（YouTube 内部 API）→ 回退到手动字幕 → 解析为纯文本。

---

## 7. 扩展系统

### 7.1 Extension 接口

```go
type Extension interface {
    Name() string
    Handle(ctx context.Context, endpoint string, method string, body []byte) (*collector.ExtensionResponse, error)
}
```

### 7.2 扩展映射表

| HTTP Endpoint | Extension | 实现要点 |
|---|---|---|
| `POST /ext/resync-source-document` | `ResyncExtension` | 根据 type 分发到 link/youtube/confluence/github/drupalwiki/paperless-ngx |
| `POST /ext/:platform-repo` | `RepoExtension` | 支持 `github` / `gitlab`，通过 API 获取文件列表和内容 |
| `POST /ext/:platform-repo/branches` | `RepoExtension` | 调用 GitHub/GitLab API 获取分支列表 |
| `POST /ext/youtube-transcript` | `YouTubeExtension` | 调用 scraper 的 YouTube 字幕提取 |
| `POST /ext/website-depth` | `WebsiteDepthExtension` | BFS 抓取指定深度，去重，限制 maxLinks |
| `POST /ext/confluence` | `ConfluenceExtension` | REST API 分页获取 space 内容 |
| `POST /ext/drupalwiki` | `DrupalWikiExtension` | REST API 获取页面 |
| `POST /ext/obsidian/vault` | `ObsidianExtension` | 本地文件系统遍历 markdown 文件 |
| `POST /ext/paperless-ngx` | `PaperlessExtension` | REST API 获取文档内容和元数据 |

### 7.3 RepoLoader

使用 `github.com/google/go-github/v63` 和 `github.com/xanzy/go-gitlab`：
- 递归获取仓库文件树
- 过滤（扩展名、大小、.gitignore）
- 下载文件内容后调用 `pipeline.ProcessFile`

---

## 8. 支撑设施

### 8.1 Tokenizer

使用 `github.com/pkoukk/tiktoken-go`，保留 Node 版的 10KB 截断估算逻辑：

```go
func (t *Tokenizer) Count(input string) int {
    const maxKBEstimate = 10 * 1024
    const divisor = 8
    if len(input) > maxKBEstimate {
        return (len(input) + divisor - 1) / divisor
    }
    return len(t.encoder.Encode(input, nil, nil))
}
```

### 8.2 文件写入（兼容 Node 版语义）

```go
func WriteToServerDocuments(storageDir string, data *collector.Document, filename string, opts WriteOptions) (*collector.Document, error) {
    var dest string
    switch {
    case opts.DestinationOverride != "":
        dest = opts.DestinationOverride
    case opts.ParseOnly:
        dest = filepath.Join(storageDir, "direct-uploads")
    default:
        dest = filepath.Join(storageDir, "documents", "custom-documents")
    }
    os.MkdirAll(dest, 0755)
    safeName := sanitizeFileName(filename) + ".json"
    path := filepath.Join(dest, safeName)
    jsonData, _ := json.MarshalIndent(data, "", "    ")
    os.WriteFile(path, jsonData, 0644)
    // location 保留最后两级路径
    parts := strings.Split(filepath.ToSlash(path), "/")
    data.Location = strings.Join(parts[len(parts)-2:], "/")
    data.IsDirectUpload = opts.ParseOnly
    return data, nil
}
```

### 8.3 外部工具适配器

**TesseractAdapter**：运行时检查 `tesseract` 是否在 PATH。命令：`tesseract <image> stdout -l <langs>`。

**WhisperAdapter**：运行时检查 `whisper` 或 `main` (whisper.cpp) 是否在 PATH。命令：`./main -m <model> -f <wav>`。

**OpenAIWhisperAdapter**：标准 multipart POST 到 `https://api.openai.com/v1/audio/transcriptions`。

**ChromedpAdapter**：`chromedp.Run` 导航 → 等待 body → `document.body.innerText`。

---

## 9. 与现有 backend 的集成

### 9.1 兼容性矩阵

| 调用点 | 文件 | 兼容性 |
|---|---|---|
| `s.coll.ProcessDocument(ctx, destPath, nil)` | `document_service.go:77` | ✓ 签名不变 |
| `s.coll.ProcessLink(ctx, link, nil, nil)` | `document_service.go:321` | ✓ 签名不变 |
| `h.coll.AcceptedFileTypes(ctx)` | `system.go:523` | ✓ 签名不变 |
| `h.coll.Online(ctx)` | `system.go:634` | ✓ 返回 true |
| `h.coll.ForwardExtensionRequest(...)` | handlers/extensions | ✓ 签名不变 |

### 9.2 main.go 修改

```go
// 修改前
coll, err := collector.NewClient(cfg.StorageDir)

// 修改后
coll, err := collector.NewLocalCollector(cfg.StorageDir, cfg)
```

### 9.3 初始化流程

`NewLocalCollector` 按以下顺序初始化：
1. Tokenizer
2. 外部工具适配器（tesseract、whisper、openAI、browser）
3. 注册所有处理器到 `processors.Registry`
4. 初始化 `scraper.Manager`
5. 注册所有扩展到 `extensions.Registry`
6. 组装 `pipeline.Pipeline`
7. 返回 `*Client`

---

## 10. 新增依赖清单

| 包名 | 用途 | 许可 |
|---|---|---|
| `github.com/pkoukk/tiktoken-go` | Token 计数 | MIT |
| `github.com/fumiama/go-docx` | DOCX 读取 | MIT |
| `github.com/qax-os/excelize/v2` | XLSX 读取 | BSD-3 |
| `github.com/chromedp/chromedp` | 浏览器抓取 | MIT |
| `github.com/PuerkitoBio/goquery` | HTML 解析 | BSD-3 |
| `github.com/google/go-github/v63` | GitHub API | BSD-3 |
| `github.com/xanzy/go-gitlab` | GitLab API | Apache-2.0 |
| `github.com/ledongthuc/pdf` | 纯 Go PDF 文本 | MIT |

---

## 11. 外部运行时依赖

| 工具 | 用途 | 必需？ |
|---|---|---|
| `tesseract` + 语言包 | 图片 OCR、扫描版 PDF OCR | 否（OCR 功能降级） |
| `pdftotext` (poppler-utils) | PDF 文本提取 fallback | 否（回退到 OCR 或失败） |
| `ffmpeg` | 音频格式转码 | 是（音频处理必需） |
| `whisper` / `whisper.cpp` | 本地音频转录 | 否（可用 OpenAI API） |
| `chromium` / `chrome` | 浏览器抓取 | 否（HTTP 回退） |

运行时检查策略：初始化时检测工具是否在 PATH，不存在则对应功能返回 `ErrOCRFailed` / `ErrTranscriptionFailed` 等错误，不影响其他功能。

---

## 12. 测试策略

1. **单元测试**：每个 `ContentExtractor` 独立测试，使用 `testdata/` 中的真实文件
2. **流水线测试**：测试完整 `ProcessFile` 流程（安全检查 → 路由 → 提取 → 丰富 → 写入）
3. **Scraper 测试**：mock HTTP server 测试通用抓取，mock chromedp 测试浏览器抓取
4. **扩展测试**：mock GitHub/GitLab/Confluence API 测试扩展
5. **集成测试**：端到端测试文件上传 → 处理 → 存储
6. **-race**：所有测试在 `-race` 下运行

---

## 13. 风险评估

| 风险 | 缓解措施 |
|---|---|
| PDF 纯 Go 提取质量差 | 三层回退（纯 Go → pdftotext → OCR） |
| DOCX/PPTX 解析不完整 | 选择成熟库（go-docx、excelize），fallback 到文本提取 |
| tesseract/whisper 未安装 | 运行时检测，优雅降级，文档明确说明 |
| chromedp 在 Windows 上不稳定 | 优先 HTTP 抓取，browser 作为可选回退 |
| 7857 行 JS 翻译遗漏 | 逐功能对照测试，保留 Node collector 作为回归基准 |

---

*设计日期: 2026-05-27*
*作者: Kimi Code CLI*
