# Document Creation — Design Spec

**Date:** 2026-05-20
**Scope:** Port AnythingLLM's "create-files" agent skill into Hermind
**Approach:** Aggregated toolset (mirrors `filesystem` pattern)

---

## 1. Goals

- Allow the agent to generate 5 document types on demand: text, Word, PowerPoint, PDF, Excel.
- Generated files are downloadable via a card in the chat UI.
- Each file type can be independently enabled/disabled in the Tools settings.
- Text/Excel/PDF are pure Go. Word/PowerPoint reuse AnythingLLM's mature Node.js libraries via a subprocess.

## 2. Non-Goals

- Real-time collaborative editing of generated files.
- Automatic cloud upload (Google Drive, OneDrive, etc.).
- OCR or image embedding inside documents (out of scope for v1).

## 3. Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Hermind Backend (Go)                    │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │  Tool Registry  │───▶│ activeToolReg() │───▶ Engine     │
│  │                 │    │ (filter by cfg) │                │
│  │  create_text_   │    └─────────────────┘                │
│  │  create_word_   │                                       │
│  │  create_pptx_   │    ┌─────────────────┐                │
│  │  create_pdf_    │───▶│ FileGenerator   │                │
│  │  create_excel_  │    │                 │                │
│  └─────────────────┘    │  ┌───────────┐  │                │
│                         │  │ Go impl   │  │                │
│                         │  │ (txt/xlsx │  │                │
│  ┌─────────────────┐    │  │  /pdf)    │  │                │
│  │ Download API    │◀───│  └───────────┘  │                │
│  │ /api/generated- │    │  ┌───────────┐  │                │
│  │    files/:name  │◀───│  │ Node.js   │  │                │
│  └─────────────────┘    │  │ subprocess│  │                │
│                         │  │ (docx/pptx)│  │                │
│  <instance>/generated-  │  └───────────┘  │                │
│      files/             └─────────────────┘                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Hermind Frontend (React)                 │
│  ┌─────────────────┐    ┌─────────────────┐                │
│  │ Tools Config    │    │ Chat Message    │                │
│  │ (5 toggles)     │    │ FileDownloadCard│                │
│  └─────────────────┘    └─────────────────┘                │
└─────────────────────────────────────────────────────────────┘
```

## 4. Backend Design

### 4.1 Tool Registry Entries

Five tools are registered in `cli/engine_deps.go`, all with `Toolset: "document_creation"`.

| Name | Generation | Dependencies |
|---|---|---|
| `create_text_file` | Go standard library (`os.WriteFile`) | None |
| `create_excel_spreadsheet` | `github.com/xuri/excelize/v2` | Go library |
| `create_pdf_document` | `github.com/go-pdf/fpdf` | Go library |
| `create_word_document` | Node.js subprocess | `docx` npm package |
| `create_pptx_presentation` | Node.js subprocess | `pptx` npm package |

Each `tool.Entry` includes:
- `Name`, `Toolset`, `Description`, `Emoji`
- `Schema`: JSON schema for LLM parameter validation
- `Handler`: Go function that generates the file and returns metadata
- `CheckFn`: For Word/PPT, returns `false` if Node.js or the script is not available

### 4.2 API Aggregation (mirrors `filesystem`)

`api/handlers_tools.go` aggregates the five `document_creation` tools into a single DTO:

```go
ToolDTO{
    Name:        "document_creation",
    Description: "Document creation — allows the agent to generate text files, Word documents, PowerPoint presentations, PDFs, and Excel spreadsheets.",
    Toolset:     "document_creation",
    Enabled:     !disabled["document_creation"],
    SettingsSchema: []ConfigFieldDTO{
        {Name: "create_text_file",         Label: "Text files",         Kind: "bool", Help: "Create text files (.txt, .md, .json, .csv, etc.)", Default: true},
        {Name: "create_word_document",     Label: "Word documents",     Kind: "bool", Help: "Create Microsoft Word documents (.docx)", Default: true},
        {Name: "create_pptx_presentation", Label: "PowerPoint",         Kind: "bool", Help: "Create PowerPoint presentations (.pptx)", Default: true},
        {Name: "create_pdf_document",      Label: "PDF documents",      Kind: "bool", Help: "Create PDF documents", Default: true},
        {Name: "create_excel_spreadsheet", Label: "Excel spreadsheets", Kind: "bool", Help: "Create Excel spreadsheets (.xlsx)", Default: true},
    },
}
```

### 4.3 activeToolReg Filtering

`api/server.go` adds a `document_creation` branch in `activeToolReg()`, symmetric to the existing `filesystem` logic:

1. Check master toggle `disabled["document_creation"]`.
2. If enabled, read per-subtype booleans from `s.opts.Config.Tools.Settings["document_creation"]`.
3. Skip tools whose subtype is disabled.
4. Skip tools whose `CheckFn` returns false (e.g., Node.js not installed).

### 4.4 File Generators

#### 4.4.1 Text File (`create_text_file`)

Parameters: `filename`, `extension`, `content`

- Normalize extension (strip leading dot, default to `txt`).
- Append extension if `filename` lacks one.
- Write UTF-8 bytes directly to `<instance>/generated-files/text-{uuid}.{ext}`.

#### 4.4.2 Excel Spreadsheet (`create_excel_spreadsheet`)

Parameters: `filename`, `title`, `content`

- Parse `content` as Markdown tables using a lightweight Markdown table parser.
- Each table becomes a worksheet. Multiple tables are separated into multiple sheets.
- Sheet name defaults to the table's preceding H2/H3 header, or "Sheet1", "Sheet2", etc.
- Save via `excelize` to `<instance>/generated-files/xlsx-{uuid}.xlsx`.

#### 4.4.3 PDF Document (`create_pdf_document`)

Parameters: `filename`, `title`, `content`

- Parse `content` as Markdown:
  - `# Heading` → large bold text
  - `##/###` → medium bold text
  - `**bold**` → bold
  - `- list item` → bulleted list
  - `| table |` → table (simplified rendering)
  - plain text → normal paragraph
- Use `fpdf` to render pages with automatic pagination.
- Add page numbers in footer.
- Save to `<instance>/generated-files/pdf-{uuid}.pdf`.

#### 4.4.4 Word Document (`create_word_document`)

Parameters: `filename`, `title`, `content` (Markdown), `theme`, `margins`, `includeTitlePage`

- Go handler serializes parameters to JSON.
- Spawns Node.js subprocess:
  ```go
  cmd := exec.CommandContext(ctx, "node", scriptPath, "docx")
  cmd.Stdin = strings.NewReader(jsonArgs)
  out, err := cmd.Output()
  ```
- Node.js script reuses AnythingLLM's `create-docx-file.js` and `utils.js` logic:
  - Parse Markdown via `marked`
  - Convert HTML to docx elements
  - Apply theme colors, margins, optional title page
  - Save via `docx` library's `Packer.toBuffer`
- Script writes to `<instance>/generated-files/` and prints the storage filename to stdout.
- Go handler reads stdout, parses the path, and returns metadata.

#### 4.4.5 PowerPoint Presentation (`create_pptx_presentation`)

Parameters: `filename`, `title`, `theme`, `sections` (array of `{title, keyPoints, instructions}`)

- Same subprocess pattern as Word, but with `type: "pptx"`.
- Reuses AnythingLLM's `create-presentation.js`, `section-agent.js`, and theme utilities.
- The "section sub-agent" research feature is **disabled** in v1 (it requires an LLM client inside the Node.js script). Each section is rendered directly from the provided `keyPoints`.
- In a future version, the sub-agent research can be re-enabled by passing an LLM API config to the Node.js script.

### 4.5 File Storage

**Path**: `<instance-root>/generated-files/`

**Filename format**: `{type}-{uuid}.{ext}`
- Example: `docx-a1b2c3d4-e5f6-7890-abcd-ef1234567890.docx`
- Regex for validation: `^([a-z]+)-([a-f0-9-]{36})\.(\w+)$`

**Directory initialization**: Server startup ensures the directory exists via `os.MkdirAll`.

### 4.6 Download API

New handler: `api/handlers_generated_files.go`

```
GET /api/generated-files/:filename
```

1. Extract `filename` from URL parameter.
2. Validate filename format against regex (defense against path traversal).
3. Check file exists under `<instance>/generated-files/`.
4. Derive MIME type from extension.
5. Set headers:
   - `Content-Type: <mime>`
   - `Content-Disposition: attachment; filename="<display-filename>"`
   - `Content-Length: <size>`
6. Stream file content.

**Security**: Reuses Hermind's existing API authentication. There is no per-conversation/workspace authorization (Hermind does not have workspaces). Any authenticated user can download any generated file.

### 4.7 Handler Return Format

All five handlers return a JSON string:

```json
{
  "filename": "report.docx",
  "storageFilename": "docx-a1b2c3d4.docx",
  "fileSize": 15234,
  "downloadUrl": "/api/generated-files/docx-a1b2c3d4.docx",
  "message": "Successfully created Word document 'report.docx' (14.9KB)."
}
```

The engine emits this as a tool result. A downstream component inspects the JSON and, if it contains `storageFilename`, emits a `file_download_card` SSE event.

## 5. Frontend Design

### 5.1 File Download Card

New component: `web/src/components/chat/FileDownloadCard.tsx`

```tsx
interface FileDownloadCardProps {
  filename: string;        // user-friendly name, e.g. "report.docx"
  storageFilename: string; // server filename, e.g. "docx-a1b2c3d4.docx"
  fileSize: number;        // bytes
}
```

Rendering:
- Icon based on extension (📄 📊 📑 📝)
- Display filename + human-readable size (e.g., "14.9 KB")
- "Download" button → `fetch(/api/generated-files/${storageFilename})`

### 5.2 Chat Integration

The engine's SSE stream includes a new event type:

```json
{
  "type": "file_download_card",
  "payload": {
    "filename": "report.docx",
    "storageFilename": "docx-a1b2c3d4.docx",
    "fileSize": 15234
  }
}
```

`ChatMessage` renders `file_download_card` payloads inline using `FileDownloadCard`.

### 5.3 Tools Configuration

The aggregated `document_creation` DTO is rendered by the existing `ToolDetailFallback` component. Because `SettingsSchema` contains five `bool` fields, the UI automatically renders five toggle switches — matching the AnythingLLM screenshot's layout without a custom detail renderer.

No new detail renderer is required for v1. A custom renderer can be added later for richer presentation (icons, grouped layout).

## 6. Node.js Subprocess Package

Extracted from AnythingLLM into `document-scripts/` at the project root:

```
document-scripts/
├── package.json              # deps: docx, pptx, marked, uuid
├── bin/generate-doc.js       # CLI: reads JSON from stdin, prints path to stdout
├── lib/
│   ├── manager.js            # simplified CreateFilesManager (save + filename utils)
│   ├── docx/
│   │   ├── create.js         # ported from create-docx-file.js
│   │   └── utils.js          # ported from docx/utils.js
│   └── pptx/
│       ├── create.js         # ported from create-presentation.js (no sub-agent)
│       └── utils.js          # ported from pptx/utils.js + themes.js
```

**Installation**:
```bash
cd document-scripts && npm install
```

**CLI contract**:
- Stdin: JSON object with `type` ("docx" or "pptx") and type-specific parameters.
- Stdout: absolute path to the generated file.
- Stderr: logs for debugging.
- Exit code 0 on success, non-zero on failure.

## 7. Error Handling

| Scenario | Strategy |
|---|---|
| Node.js / scripts not installed | `CheckFn` returns `false`; tools are hidden from LLM. Frontend toggle shows grayed-out hint. |
| Node.js subprocess exits with error | Handler returns `{"error": "..."}` to LLM. No download card is sent. |
| File write fails | Same as above. |
| Download request with invalid filename | HTTP 400 |
| Download request for missing file | HTTP 404 |
| Markdown parse error in Go generators | Fall back to plain text rendering. |

## 8. Testing

| Layer | What |
|---|---|
| Unit | Go text/Excel/PDF generators: input → expected file structure |
| Integration | Node.js subprocess round-trip: JSON args → file exists → valid format |
| API | `GET /api/tools` returns aggregated `document_creation` with 5 schema fields. `GET /api/generated-files/:name` downloads correctly. `activeToolReg()` respects master + subtype toggles. |
| Frontend | `FileDownloadCard` renders filename, size, and download button. Click triggers fetch. |
| E2E | Full flow: agent creates file → chat shows download card → user clicks → file downloads. |

CI note: Word/PPT integration tests are skipped if `node` is not in `$PATH`.

## 9. File Inventory

### New files

```
backend:
  api/handlers_generated_files.go
  api/handlers_generated_files_test.go
  config/descriptor/document_creation.go
  tool/document/
    ├── register.go
    ├── text.go
    ├── text_test.go
    ├── excel.go
    ├── excel_test.go
    ├── pdf.go
    ├── pdf_test.go
    ├── nodejs.go
    └── nodejs_test.go

frontend:
  web/src/components/chat/FileDownloadCard.tsx
  web/src/components/chat/FileDownloadCard.module.css
  web/src/components/chat/FileDownloadCard.test.tsx

document-scripts/:
  package.json
  bin/generate-doc.js
  lib/manager.js
  lib/docx/create.js
  lib/docx/utils.js
  lib/pptx/create.js
  lib/pptx/utils.js
```

### Modified files

```
backend:
  api/handlers_tools.go          # add document_creation aggregation
  api/handlers_tools_test.go     # add tests for aggregation
  api/server.go                  # add activeToolReg filtering
  cli/engine_deps.go             # register 5 tools
  go.mod                         # add excelize, fpdf

frontend:
  web/src/components/chat/ChatMessage.tsx        # render file_download_card
  web/src/api/schemas.ts                          # add FileDownloadCard event type
```

## 10. Open Questions (v2)

- **File cleanup**: Auto-delete files older than N days. Config key: `generated_files_max_age_days`.
- **PPT sub-agent research**: Re-enable AnythingLLM's section sub-agent by passing an LLM config to the Node.js script.
- **Custom detail renderer**: Richer Tools config UI with icons and grouped layout.
- **Image embedding**: Allow the agent to embed images inside documents.
