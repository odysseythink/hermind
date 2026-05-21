# Bundle Node.js Runtime with desktop-electron Installer

## Background

Hermind's `document_creation` toolset includes five sub-tools:

| Tool | Implementation | Needs Node.js |
|------|----------------|---------------|
| `create_text_file` | Pure Go | No |
| `create_excel_spreadsheet` | Pure Go (`excelize`) | No |
| `create_pdf_document` | Pure Go (`fpdf`) | No |
| `create_word_document` | Node.js subprocess | **Yes** |
| `create_pptx_presentation` | Node.js subprocess | **Yes** |

The two Node.js-based tools (`create_word_document`, `create_pptx_presentation`) invoke a Node.js script (`document-scripts/bin/generate-doc.js`) via Go's `os/exec`. Today this requires the end user to have Node.js installed on their system and available in `PATH`.

**Goal:** Bundle a Node.js runtime and the `document-scripts` package into the desktop-electron installer so that users do not need to install Node.js separately.

## Constraints

- Windows is the primary target (NSIS + portable builds configured in `electron-builder.yml`).
- The solution must work offline after installation.
- The existing Go backend (`hermind-desktop.exe`) must continue to work in dev mode (where Node.js may come from the developer's system PATH).
- The `document-scripts/bin/generate-doc.js` entry point is currently missing and must be created as part of this work.

## Approach Overview

Use **electron-builder** `extraResources` and `extraFiles` to bundle the Node.js runtime and `document-scripts` into the installer. Modify the Go backend's `NodeJSWrapper` to locate the bundled `node.exe` before falling back to the system `PATH`.

## Post-Installation Directory Structure

```
D:\tools\Hermind\                      (installation root)
├── Hermind.exe                         # Electron main executable
├── hermind-desktop.exe                 # Go backend (extraResources)
├── node.exe                            # Node.js runtime (extraResources)
├── document-scripts\                   # JS scripts + dependencies (extraFiles)
│   ├── bin\generate-doc.js
│   ├── lib\...
│   └── node_modules\...
├── browser-extension\                  # (existing extraFiles)
├── resources\                          # Electron runtime resources
│   └── icon.ico
└── .hermind\                           # instanceRoot (created at runtime)
    ├── config.yaml
    ├── state.db
    └── generated-files\                # Output directory for created documents
```

### Path Relationship Verification

- `hermind-desktop.exe` lands in `resources/` via `extraResources`. Go backend locates itself with `os.Executable()` → `.../resources/hermind-desktop.exe`. The bundled `node.exe` is placed in the same `resources/` directory, so it is discoverable relative to the executable.
- `document-scripts` lands in the installation root via `extraFiles`. The Go backend computes `scriptDir = filepath.Join(instanceRoot, "..", "document-scripts")`. Since `instanceRoot` is `.../.hermind/`, `../document-scripts` resolves to `.../document-scripts`, matching the placement exactly.

## Detailed Design

### 1. Build Preparation Script

Add `desktop-electron/scripts/prepare-node.js`.

Responsibilities:

1. Download the Node.js Windows x64 zip (default version **v20.17.0**; overridable via `NODE_VERSION` env var).
2. Extract `node.exe` from the zip and place it at `desktop-electron/resources/node.exe`.
3. Copy the project root `document-scripts/` directory into `desktop-electron/resources/document-scripts/`.
4. Run `npm install --production` inside the copied `document-scripts/` directory.
5. Verify that `bin/generate-doc.js` exists inside the copied tree (it will be added as a source file in the project root `document-scripts/`; see Section 4).

Integration into `package.json`:

```json
{
  "scripts": {
    "build:go": "go build -ldflags=\"-s -w\" -o resources/hermind-desktop.exe ../cmd/hermind",
    "prepare:node": "node scripts/prepare-node.js",
    "dist": "npm run build:go && npm run prepare:node && npm run build && electron-builder",
    "dist:dir": "npm run build:go && npm run prepare:node && npm run build && electron-builder --dir"
  }
}
```

### 2. Electron-Builder Configuration

Update `desktop-electron/electron-builder.yml`:

```yaml
appId: com.odysseythink.hermind
productName: Hermind
directories:
  output: dist
files:
  - out/**/*
  - resources/**/*
extraResources:
  - from: resources/hermind-desktop.exe
    to: hermind-desktop.exe
  - from: resources/node.exe
    to: node.exe
extraFiles:
  - from: ../browser-extension
    to: browser-extension
    filter:
      - "**/*"
  - from: resources/document-scripts
    to: document-scripts
    filter:
      - "**/*"
win:
  target:
    - target: nsis
      arch: x64
    - target: portable
      arch: x64
  icon: resources/icon.ico
nsis:
  oneClick: false
  allowToChangeInstallationDirectory: true
  createDesktopShortcut: always
  artifactName: ${productName}-Setup-${version}.${ext}
portable:
  artifactName: ${productName}-Portable-${version}.${ext}
```

Mirror the same changes in `desktop-electron/package.json` under the `build` field so both YAML and JSON configurations remain consistent.

### 3. Go Backend — Node.js Discovery

Modify `tool/document/nodejs.go`.

Add a `findNodeExecutable` helper that searches in this order:

1. The directory containing `hermind-desktop.exe` (e.g., `resources/node.exe` on Windows).
2. The parent directory of the executable's directory (installation root), as a fallback.
3. System `PATH` via `exec.LookPath("node")`.

```go
func findNodeExecutable() string {
	// 1. Same directory as the Go executable
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates := []string{
			filepath.Join(exeDir, "node.exe"),
			filepath.Join(exeDir, "..", "node.exe"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c
			}
		}
	}

	// 2. Fallback to system PATH
	if path, err := exec.LookPath("node"); err == nil {
		return path
	}

	return ""
}
```

Update `Generate`:

```go
nodePath := findNodeExecutable()
if nodePath == "" {
    return "", fmt.Errorf("node.js executable not found")
}
cmd := exec.CommandContext(ctx, nodePath, scriptPath)
```

Update `IsAvailable`:

```go
func (w *NodeJSWrapper) IsAvailable() bool {
    scriptPath := filepath.Join(w.scriptDir, "bin", "generate-doc.js")
    if _, err := os.Stat(scriptPath); err != nil {
        return false
    }
    if findNodeExecutable() == "" {
        return false
    }
    return true
}
```

### 4. `document-scripts/bin/generate-doc.js` Entry Point

Create `document-scripts/bin/generate-doc.js` as a permanent source file in the repository:

```javascript
#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

async function main() {
    const input = fs.readFileSync(0, 'utf-8');
    const args = JSON.parse(input);
    const docType = args.type;
    const outputDir = args.outputDir;

    if (!docType || !outputDir) {
        console.error('Missing required fields: type, outputDir');
        process.exit(1);
    }

    let resultPath;
    if (docType === 'docx') {
        const { createDocx } = require('../lib/docx/create');
        resultPath = await createDocx(args, outputDir);
    } else if (docType === 'pptx') {
        const { createPptx } = require('../lib/pptx/create');
        resultPath = await createPptx(args, outputDir);
    } else {
        console.error(`Unsupported document type: ${docType}`);
        process.exit(1);
    }

    console.log(resultPath);
}

main().catch(err => {
    console.error(err);
    process.exit(1);
});
```

**Note:** The exact `require` paths and function signatures (`createDocx`, `createPptx`) must match the actual exports in `document-scripts/lib/docx/create.js` and `document-scripts/lib/pptx/create.js`.

## Error Handling

| Stage | Failure | Behavior |
|-------|---------|----------|
| Build | `prepare-node.js` fails to download Node.js | Script exits non-zero; `npm run dist` fails early with a clear error. |
| Build | `npm install` inside copied `document-scripts` fails | Same as above. |
| Runtime | Bundled `node.exe` missing | `IsAvailable()` returns `false`; the tool is hidden/unavailable in the UI. |
| Runtime | `document-scripts` missing | `IsAvailable()` returns `false`. |
| Runtime | Node.js subprocess crashes | Existing behavior: stderr captured and returned as a tool error to the LLM. |
| Runtime | Script returns path outside output dir | Existing behavior: rejected with security error. |

## Testing Strategy

1. **Unit tests (Go):** Add tests for `findNodeExecutable` covering:
   - Finds `node.exe` next to the executable.
   - Falls back to `PATH` when bundled binary is absent.
   - Returns empty string when neither exists.

2. **Build verification:** After running `npm run dist`, inspect the generated installer/unpacked directory and confirm:
   - `node.exe` exists in `resources/` (or the equivalent unpacked path).
   - `document-scripts/bin/generate-doc.js` exists.
   - `document-scripts/node_modules/` contains `docx`, `pptxgenjs`, etc.

3. **End-to-end (manual):** Install the built NSIS or portable package, launch the app, and invoke `create_word_document` and `create_pptx_presentation` to verify documents are generated successfully without system Node.js.

## Compatibility

- **Dev mode:** Unchanged. Developers who have Node.js on their PATH continue to work as before.
- **Portable build:** The same `extraResources`/`extraFiles` configuration applies; the portable archive will contain `node.exe` and `document-scripts`.
- **Non-Windows platforms:** This design focuses on Windows (the current desktop-electron target). If macOS/Linux builds are added later, the same approach works by bundling the platform-specific `node` binary and adjusting the executable extension lookup.
