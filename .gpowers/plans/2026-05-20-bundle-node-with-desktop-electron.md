# Bundle Node.js Runtime with desktop-electron Installer — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bundle a Node.js runtime and the `document-scripts` package into the desktop-electron installer so users do not need to install Node.js separately to use `create_word_document` and `create_pptx_presentation`.

**Architecture:** Add a build-time script (`prepare-node.js`) that downloads a portable `node.exe` and stages `document-scripts` into `desktop-electron/resources/`. Use `electron-builder` `extraResources`/`extraFiles` to bundle them. Modify the Go backend's `NodeJSWrapper` to locate the bundled `node.exe` relative to `hermind-desktop.exe` before falling back to the system `PATH`.

**Tech Stack:** Node.js (build script), Go (backend logic), electron-builder (packaging)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `document-scripts/bin/generate-doc.js` | Create | CLI entry point invoked by Go backend; reads JSON from stdin, calls docx/pptx creators, prints output path to stdout |
| `tool/document/nodejs.go` | Modify | Add `findNodeExecutable` helper; use it in `Generate` and `IsAvailable` |
| `tool/document/nodejs_test.go` | Modify | Add unit tests for `findNodeExecutable` |
| `desktop-electron/scripts/prepare-node.js` | Create | Download Node.js zip, extract `node.exe`, copy `document-scripts`, run `npm install --production` |
| `desktop-electron/package.json` | Modify | Add `prepare:node` script, update `dist`/`dist:dir`, add `extraResources`/`extraFiles` to `build` |
| `desktop-electron/electron-builder.yml` | Modify | Add `node.exe` to `extraResources`, add `document-scripts` to `extraFiles` |

---

## Task 1: Create `document-scripts/bin/generate-doc.js`

**Files:**
- Create: `document-scripts/bin/generate-doc.js`

- [ ] **Step 1: Write the entry-point script**

```javascript
#!/usr/bin/env node
const fs = require('fs');

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
        resultPath = await createDocx(args);
    } else if (docType === 'pptx') {
        const { createPptx } = require('../lib/pptx/create');
        resultPath = await createPptx(args);
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

- [ ] **Step 2: Commit**

```bash
git add document-scripts/bin/generate-doc.js
git commit -m "feat(document-scripts): add bin/generate-doc.js CLI entry point"
```

---

## Task 2: Add `findNodeExecutable` to Go backend with tests

**Files:**
- Modify: `tool/document/nodejs.go`
- Modify: `tool/document/nodejs_test.go`

- [ ] **Step 1: Write the failing test**

Add to `tool/document/nodejs_test.go` (keep existing tests intact):

```go
package document

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindNodeExecutable_BundledNextToExe(t *testing.T) {
	tmpDir := t.TempDir()
	nodeName := "node"
	if runtime.GOOS == "windows" {
		nodeName = "node.exe"
	}
	nodePath := filepath.Join(tmpDir, nodeName)
	require.NoError(t, os.WriteFile(nodePath, []byte("fake"), 0o755))

	result := findNodeExecutableFrom(filepath.Join(tmpDir, "hermind-desktop.exe"))
	require.Equal(t, nodePath, result)
}

func TestFindNodeExecutable_BundledInParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	nodeName := "node"
	if runtime.GOOS == "windows" {
		nodeName = "node.exe"
	}
	nodePath := filepath.Join(tmpDir, nodeName)
	require.NoError(t, os.WriteFile(nodePath, []byte("fake"), 0o755))

	// exe lives in tmpDir/resources/
	result := findNodeExecutableFrom(filepath.Join(tmpDir, "resources", "hermind-desktop.exe"))
	require.Equal(t, nodePath, result)
}

func TestFindNodeExecutable_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	result := findNodeExecutableFrom(filepath.Join(tmpDir, "hermind-desktop.exe"))
	// We cannot assert an exact path here because the test runner may have
	// a real node on PATH. Just assert it does not return a path inside tmpDir.
	require.NotContains(t, result, tmpDir)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd tool/document && go test -v -run TestFindNodeExecutable
```

Expected: FAIL with `findNodeExecutableFrom undefined`

- [ ] **Step 3: Implement `findNodeExecutable` and `findNodeExecutableFrom`**

Modify `tool/document/nodejs.go`. Add the helper, keep `Generate` and `IsAvailable` using it:

```go
// findNodeExecutable locates a Node.js binary.
// Priority: 1) same dir as this executable, 2) parent dir of this executable,
// 3) system PATH.
func findNodeExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	return findNodeExecutableFrom(exe)
}

func findNodeExecutableFrom(exePath string) string {
	nodeName := "node"
	if os.PathSeparator == '\\' {
		nodeName = "node.exe"
	}

	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(exeDir, nodeName),
			filepath.Join(exeDir, "..", nodeName),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				if abs, err := filepath.Abs(c); err == nil {
					return abs
				}
				return c
			}
		}
	}

	if path, err := exec.LookPath("node"); err == nil {
		return path
	}

	return ""
}
```

Update `Generate` inside `tool/document/nodejs.go`:

Replace:
```go
	cmd := exec.CommandContext(ctx, "node", scriptPath)
```

With:
```go
	nodePath := findNodeExecutable()
	if nodePath == "" {
		return "", fmt.Errorf("node.js executable not found")
	}
	cmd := exec.CommandContext(ctx, nodePath, scriptPath)
```

Update `IsAvailable` inside `tool/document/nodejs.go`:

Replace:
```go
func (w *NodeJSWrapper) IsAvailable() bool {
	scriptPath := filepath.Join(w.scriptDir, "bin", "generate-doc.js")
	if _, err := os.Stat(scriptPath); err != nil {
		return false
	}
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}
	return true
}
```

With:
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

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd tool/document && go test -v -run TestFindNodeExecutable
```

Expected: PASS

Also run the existing NodeJS tests to avoid regressions:

```bash
cd tool/document && go test -v
```

Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add tool/document/nodejs.go tool/document/nodejs_test.go
git commit -m "feat(document): locate bundled node.exe before falling back to PATH"
```

---

## Task 3: Create `desktop-electron/scripts/prepare-node.js`

**Files:**
- Create: `desktop-electron/scripts/prepare-node.js`

- [ ] **Step 1: Write the build preparation script**

```javascript
const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const NODE_VERSION = process.env.NODE_VERSION || '20.17.0';
const NODE_ZIP_URL = `https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-win-x64.zip`;

const ROOT_DIR = path.resolve(__dirname, '..');
const RESOURCES_DIR = path.join(ROOT_DIR, 'resources');
const TEMP_DIR = path.join(ROOT_DIR, '.tmp-node');
const NODE_EXE_PATH = path.join(RESOURCES_DIR, 'node.exe');
const DOC_SCRIPTS_SRC = path.resolve(ROOT_DIR, '..', 'document-scripts');
const DOC_SCRIPTS_DEST = path.join(RESOURCES_DIR, 'document-scripts');

function ensureDir(dir) {
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }
}

function downloadFile(url, dest) {
    return new Promise((resolve, reject) => {
        const file = fs.createWriteStream(dest);
        https.get(url, (response) => {
            if (response.statusCode === 301 || response.statusCode === 302) {
                downloadFile(response.headers.location, dest).then(resolve).catch(reject);
                return;
            }
            if (response.statusCode !== 200) {
                reject(new Error(`Download failed with status ${response.statusCode}: ${url}`));
                return;
            }
            response.pipe(file);
            file.on('finish', () => {
                file.close();
                resolve();
            });
        }).on('error', reject);
    });
}

async function main() {
    // Skip if node.exe already exists
    if (fs.existsSync(NODE_EXE_PATH)) {
        console.log(`node.exe already exists at ${NODE_EXE_PATH}, skipping download.`);
    } else {
        ensureDir(TEMP_DIR);
        const zipPath = path.join(TEMP_DIR, 'node.zip');

        console.log(`Downloading Node.js v${NODE_VERSION}...`);
        await downloadFile(NODE_ZIP_URL, zipPath);

        console.log('Extracting node.exe...');
        execSync(
            `powershell -Command "Expand-Archive -Path '${zipPath}' -DestinationPath '${TEMP_DIR}' -Force"`,
            { stdio: 'inherit' }
        );

        const extractedNode = path.join(TEMP_DIR, `node-v${NODE_VERSION}-win-x64`, 'node.exe');
        if (!fs.existsSync(extractedNode)) {
            throw new Error(`Expected node.exe not found at ${extractedNode}`);
        }

        fs.copyFileSync(extractedNode, NODE_EXE_PATH);
        console.log(`Copied node.exe to ${NODE_EXE_PATH}`);
    }

    // Prepare document-scripts
    if (!fs.existsSync(DOC_SCRIPTS_SRC)) {
        throw new Error(`document-scripts source not found at ${DOC_SCRIPTS_SRC}`);
    }

    if (fs.existsSync(DOC_SCRIPTS_DEST)) {
        console.log('Removing old document-scripts copy...');
        fs.rmSync(DOC_SCRIPTS_DEST, { recursive: true, force: true });
    }

    console.log('Copying document-scripts...');
    fs.cpSync(DOC_SCRIPTS_SRC, DOC_SCRIPTS_DEST, { recursive: true });

    console.log('Installing document-scripts dependencies...');
    execSync('npm install --production', {
        cwd: DOC_SCRIPTS_DEST,
        stdio: 'inherit',
    });

    // Verify bin/generate-doc.js exists
    const entryPoint = path.join(DOC_SCRIPTS_DEST, 'bin', 'generate-doc.js');
    if (!fs.existsSync(entryPoint)) {
        throw new Error(`Entry point missing: ${entryPoint}`);
    }

    // Clean up temp
    if (fs.existsSync(TEMP_DIR)) {
        fs.rmSync(TEMP_DIR, { recursive: true, force: true });
    }

    console.log('prepare-node.js completed successfully.');
}

main().catch(err => {
    console.error(err);
    process.exit(1);
});
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/scripts/prepare-node.js
git commit -m "build(electron): add prepare-node.js script for bundling Node.js runtime"
```

---

## Task 4: Update electron-builder configs

**Files:**
- Modify: `desktop-electron/package.json`
- Modify: `desktop-electron/electron-builder.yml`

- [ ] **Step 1: Update `desktop-electron/package.json`**

Update `scripts`:
```json
"scripts": {
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "build:go": "go build -ldflags=\"-s -w\" -o resources/hermind-desktop.exe ../cmd/hermind",
    "preview": "electron-vite preview",
    "prepare:node": "node scripts/prepare-node.js",
    "dist": "npm run build:go && npm run prepare:node && npm run build && electron-builder",
    "dist:dir": "npm run build:go && npm run prepare:node && npm run build && electron-builder --dir"
}
```

Update `build`:
```json
"build": {
    "appId": "com.odysseythink.hermind",
    "productName": "Hermind",
    "directories": {
      "output": "dist"
    },
    "files": [
      "out/**/*",
      "resources/**/*"
    ],
    "extraResources": [
      {
        "from": "resources/hermind-desktop.exe",
        "to": "hermind-desktop.exe"
      },
      {
        "from": "resources/node.exe",
        "to": "node.exe"
      }
    ],
    "extraFiles": [
      {
        "from": "../browser-extension",
        "to": "browser-extension",
        "filter": ["**/*"]
      },
      {
        "from": "resources/document-scripts",
        "to": "document-scripts",
        "filter": ["**/*"]
      }
    ],
    "win": {
      "target": [
        { "target": "nsis", "arch": "x64" },
        { "target": "portable", "arch": "x64" }
      ],
      "icon": "resources/icon.ico"
    },
    "nsis": {
      "oneClick": false,
      "allowToChangeInstallationDirectory": true,
      "createDesktopShortcut": "always",
      "artifactName": "${productName}-Setup-${version}.${ext}",
      "include": "build/installer.nsh"
    },
    "portable": {
      "artifactName": "${productName}-Portable-${version}.${ext}"
    }
}
```

- [ ] **Step 2: Update `desktop-electron/electron-builder.yml`**

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

- [ ] **Step 3: Commit**

```bash
git add desktop-electron/package.json desktop-electron/electron-builder.yml
git commit -m "build(electron): bundle node.exe and document-scripts into installer"
```

---

## Task 5: Build verification

**Files:**
- (no file changes; manual verification)

- [ ] **Step 1: Run `dist:dir` to produce an unpacked directory**

```bash
cd desktop-electron
npm run dist:dir
```

- [ ] **Step 2: Verify directory structure**

Check that the unpacked directory (e.g., `dist/win-unpacked/`) contains:

```
dist/win-unpacked/
├── Hermind.exe
├── hermind-desktop.exe          ← in resources/ (extraResources)
├── node.exe                     ← in resources/ (extraResources)
├── document-scripts\            ← in root (extraFiles)
│   ├── bin\generate-doc.js
│   ├── lib\...
│   └── node_modules\...
└── browser-extension\           ← in root (extraFiles)
```

- [ ] **Step 3: Verify `node.exe` runs**

```bash
.\dist\win-unpacked\resources\node.exe --version
```

Expected output: `v20.17.0` (or the configured version)

- [ ] **Step 4: Verify `generate-doc.js` works standalone**

```bash
cd dist/win-unpacked/document-scripts
$env:NODE_PATH="./node_modules"; .\..\resources\node.exe .\bin\generate-doc.js
```

Type the following JSON and press Ctrl+D (or pipe it):
```json
{"type":"docx","outputDir":"C:\\temp","filename":"test.docx","content":"# Hello","title":"Test"}
```

Expected: a file path printed to stdout, and the `.docx` file created at that path.

- [ ] **Step 5: Run a full `npm run dist` (optional, for final installer)**

If the unpacked verification passes, run the full build to produce the NSIS installer:

```bash
cd desktop-electron
npm run dist
```

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] Bundle Node.js runtime into installer → Task 3 + Task 4
- [x] Bundle `document-scripts` with dependencies → Task 3 + Task 4
- [x] Go backend locates bundled `node.exe` → Task 2
- [x] Fallback to system PATH when bundled binary absent → Task 2
- [x] Missing `bin/generate-doc.js` created → Task 1
- [x] Dev mode unchanged → Task 2 (fallback PATH logic preserves this)

**2. Placeholder scan:**
- [x] No TBD/TODO/fill-in-details
- [x] No vague "add error handling" without code
- [x] No "similar to Task N" references
- [x] All file paths are exact

**3. Type consistency:**
- [x] `findNodeExecutable` / `findNodeExecutableFrom` naming consistent
- [x] `nodeName` platform detection (`node` vs `node.exe`) consistent between implementation and tests
- [x] `params` shape passed to `createDocx`/`createPptx` matches their actual signatures
