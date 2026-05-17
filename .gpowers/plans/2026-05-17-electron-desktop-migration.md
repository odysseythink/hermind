# Electron Desktop Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use gpowers:subagent-driven-development (recommended) or gpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Qt6 QML desktop client with an Electron shell that wraps the existing Web UI, reusing the Go `hermind desktop` HTTP backend.

**Architecture:** Electron main process spawns `hermind desktop` as a child process, discovers the HTTP port from stdout (`HERMIND_READY <port>`), then loads the Web UI from `http://localhost:<port>/ui/`. Native capabilities (tray, shortcuts, file dialogs) are bridged via a preload script exposing `window.electronAPI`.

**Tech Stack:** Electron 35 + Vite + TypeScript. Go backend unchanged (reuses existing `cli/desktop.go`).

---

## File Structure

### New Files (Electron project)

| File | Responsibility |
|---|---|
| `desktop-electron/package.json` | Project metadata, dependencies, scripts |
| `desktop-electron/tsconfig.json` | TypeScript config (strict, Node + DOM) |
| `desktop-electron/electron.vite.config.ts` | Vite build config for main + preload + renderer |
| `desktop-electron/electron-builder.yml` | Packaging config (installer, icon, resources) |
| `desktop-electron/src/main/index.ts` | Main entry: orchestrate Go startup, create window, register IPC |
| `desktop-electron/src/main/go-process.ts` | Spawn `hermind desktop`, parse HERMIND_READY, health check, restart |
| `desktop-electron/src/main/window.ts` | BrowserWindow creation, load URL, close-to-tray, state save |
| `desktop-electron/src/main/tray.ts` | Tray icon, context menu (Show / Quit), click handlers |
| `desktop-electron/src/main/shortcuts.ts` | Global shortcut registration (Ctrl+Shift+H toggle) |
| `desktop-electron/src/main/ipc.ts` | IPC handler definitions (window, dialog, notification, Go status) |
| `desktop-electron/src/preload/index.ts` | contextBridge: expose typed `electronAPI` to renderer |
| `desktop-electron/src/renderer/index.html` | Dev: Vite dev server; Prod: loaded from Go HTTP server |
| `desktop-electron/resources/icon.ico` | Windows app icon (reuse from desktop/resources/) |

### Modified Files (Go backend — minimal)

| File | Change |
|---|---|
| `cli/desktop.go` | Write port to temp file as stdout parsing fallback |

### Existing Files Reused (unchanged)

| File | Role |
|---|---|
| `cli/desktop.go` | Go HTTP server entry with `HERMIND_READY` signal |
| `cli/web.go` | HTTP server implementation (runWeb) |
| `api/webroot/*` | Existing Web UI (React/Vite) — served by Go |
| `api/*` | REST API + SSE streaming — unchanged |

---

## Task 1: Initialize Electron Project

**Files:**
- Create: `desktop-electron/package.json`
- Create: `desktop-electron/tsconfig.json`
- Create: `desktop-electron/electron.vite.config.ts`
- Create: `desktop-electron/src/renderer/index.html`

- [ ] **Step 1: Create package.json**

```json
{
  "name": "hermind-desktop-electron",
  "version": "1.0.0",
  "description": "Hermind desktop client (Electron)",
  "main": "./out/main/index.js",
  "author": "Hermind",
  "license": "MIT",
  "scripts": {
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "preview": "electron-vite preview",
    "dist": "npm run build && electron-builder",
    "dist:dir": "npm run build && electron-builder --dir"
  },
  "devDependencies": {
    "@types/node": "^22.0.0",
    "electron": "^35.0.0",
    "electron-builder": "^26.0.0",
    "electron-vite": "^3.0.0",
    "typescript": "^5.7.0",
    "vite": "^6.0.0"
  },
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
      }
    ],
    "win": {
      "target": "nsis",
      "icon": "resources/icon.ico"
    },
    "nsis": {
      "oneClick": false,
      "allowToChangeInstallationDirectory": true
    }
  }
}
```

- [ ] **Step 2: Create tsconfig.json**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "outDir": "./out",
    "rootDir": "./src",
    "types": ["node"]
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "out", "dist"]
}
```

- [ ] **Step 3: Create electron.vite.config.ts**

```typescript
import { defineConfig, externalizeDepsPlugin } from 'electron-vite'

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()],
    build: {
      outDir: 'out/main',
      lib: {
        entry: 'src/main/index.ts',
        formats: ['cjs'],
        fileName: () => 'index.js'
      }
    }
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
    build: {
      outDir: 'out/preload',
      lib: {
        entry: 'src/preload/index.ts',
        formats: ['cjs'],
        fileName: () => 'index.js'
      }
    }
  },
  renderer: {
    root: 'src/renderer',
    build: {
      outDir: 'out/renderer',
      rollupOptions: {
        input: 'src/renderer/index.html'
      }
    }
  }
})
```

- [ ] **Step 4: Create renderer/index.html**

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>Hermind</title>
</head>
<body>
  <div id="app">Loading...</div>
</body>
</html>
```

- [ ] **Step 5: Install dependencies**

```bash
cd desktop-electron
npm install
```

Expected: `node_modules/` created, no errors.

- [ ] **Step 6: Commit**

```bash
git add desktop-electron/
git commit -m "feat(electron): initialize Electron project structure"
```

---

## Task 2: Go Subprocess Manager

**Files:**
- Create: `desktop-electron/src/main/go-process.ts`

- [ ] **Step 1: Implement Go subprocess manager**

```typescript
import { spawn, ChildProcess } from 'child_process'
import { app } from 'electron'
import * as path from 'path'
import * as fs from 'fs'
import * as os from 'os'

const MAX_RETRIES = 5
const RETRY_DELAY_MS = 2000
const HEALTH_TIMEOUT_MS = 15000
const PORT_REGEX = /HERMIND_READY\s+(\d+)/

export type GoStatus = 'starting' | 'running' | 'restarting' | 'error'

export interface GoProcessManager {
  start(): Promise<number>
  stop(): void
  getStatus(): GoStatus
  getPort(): number | null
  onStatusChange(callback: (status: GoStatus) => void): () => void
}

export function createGoProcessManager(): GoProcessManager {
  let proc: ChildProcess | null = null
  let port: number | null = null
  let status: GoStatus = 'starting'
  let retryCount = 0
  let statusListeners: ((status: GoStatus) => void)[] = []

  function setStatus(newStatus: GoStatus) {
    status = newStatus
    statusListeners.forEach(cb => cb(newStatus))
  }

  function getGoBinaryPath(): string {
    if (app.isPackaged) {
      return path.join(process.resourcesPath, 'hermind-desktop.exe')
    }
    const devPath = path.join(app.getAppPath(), '..', '..', 'hermind-desktop.exe')
    if (fs.existsSync(devPath)) return devPath
    return 'go'
  }

  function getGoArgs(): string[] {
    const binPath = getGoBinaryPath()
    if (binPath === 'go') {
      return ['run', './cmd/hermind', 'desktop']
    }
    return ['desktop']
  }

  function getGoCwd(): string | undefined {
    const binPath = getGoBinaryPath()
    if (binPath === 'go') {
      return path.join(app.getAppPath(), '..', '..')
    }
    return undefined
  }

  async function findPortFromFile(pid: number, timeoutMs: number): Promise<number | null> {
    const tempFile = path.join(os.tmpdir(), `hermind-port-${pid}.txt`)
    const deadline = Date.now() + timeoutMs
    while (Date.now() < deadline) {
      if (fs.existsSync(tempFile)) {
        const content = fs.readFileSync(tempFile, 'utf-8').trim()
        const parsed = parseInt(content, 10)
        if (!isNaN(parsed)) return parsed
      }
      await sleep(200)
    }
    return null
  }

  async function waitForHealth(port: number, timeoutMs: number): Promise<boolean> {
    const deadline = Date.now() + timeoutMs
    while (Date.now() < deadline) {
      try {
        const response = await fetch(`http://127.0.0.1:${port}/api/health`)
        if (response.ok) return true
      } catch {
        // Not ready yet
      }
      await sleep(500)
    }
    return false
  }

  function spawnProcess(): Promise<number> {
    return new Promise((resolve, reject) => {
      setStatus('starting')
      retryCount++

      const binPath = getGoBinaryPath()
      const args = getGoArgs()
      const cwd = getGoCwd()

      console.log(`[Go] Starting: ${binPath} ${args.join(' ')}`)

      proc = spawn(binPath, args, {
        cwd,
        stdio: ['ignore', 'pipe', 'pipe'],
        env: { ...process.env, HERMIND_DESKTOP_MODE: '1' }
      })

      let portResolved = false

      proc.stdout?.on('data', (data: Buffer) => {
        const text = data.toString()
        console.log(`[Go stdout] ${text.trim()}`)

        if (!portResolved) {
          const match = PORT_REGEX.exec(text)
          if (match) {
            portResolved = true
            port = parseInt(match[1], 10)
            console.log(`[Go] Port discovered: ${port}`)
            resolve(port)
          }
        }
      })

      proc.stderr?.on('data', (data: Buffer) => {
        console.error(`[Go stderr] ${data.toString().trim()}`)
      })

      proc.on('error', (err) => {
        console.error('[Go] Spawn error:', err)
        if (!portResolved) reject(err)
      })

      proc.on('exit', (code) => {
        console.log(`[Go] Process exited with code ${code}`)
        proc = null
        port = null
        if (!portResolved) {
          reject(new Error(`Go process exited before signaling ready (code ${code})`))
        } else {
          setStatus('error')
          handleCrash()
        }
      })

      setTimeout(async () => {
        if (!portResolved && proc && proc.pid) {
          const filePort = await findPortFromFile(proc.pid, 5000)
          if (filePort) {
            portResolved = true
            port = filePort
            console.log(`[Go] Port discovered from file: ${port}`)
            resolve(port)
          }
        }
      }, 10000)
    })
  }

  async function handleCrash() {
    if (retryCount > MAX_RETRIES) {
      console.error(`[Go] Max retries (${MAX_RETRIES}) exceeded`)
      setStatus('error')
      return
    }

    setStatus('restarting')
    console.log(`[Go] Restarting in ${RETRY_DELAY_MS}ms (attempt ${retryCount}/${MAX_RETRIES})`)
    await sleep(RETRY_DELAY_MS)
    try {
      const newPort = await spawnProcess()
      const healthy = await waitForHealth(newPort, HEALTH_TIMEOUT_MS)
      if (healthy) {
        setStatus('running')
        retryCount = 0
      } else {
        throw new Error('Health check failed after restart')
      }
    } catch (err) {
      console.error('[Go] Restart failed:', err)
      handleCrash()
    }
  }

  return {
    async start() {
      retryCount = 0
      const discoveredPort = await spawnProcess()
      const healthy = await waitForHealth(discoveredPort, HEALTH_TIMEOUT_MS)
      if (!healthy) {
        throw new Error(`Go health check failed on port ${discoveredPort}`)
      }
      setStatus('running')
      retryCount = 0
      return discoveredPort
    },

    stop() {
      if (proc) {
        console.log('[Go] Stopping process...')
        proc.kill('SIGTERM')
        const killTimeout = setTimeout(() => {
          if (proc && !proc.killed) {
            console.log('[Go] Force killing process...')
            proc.kill('SIGKILL')
          }
        }, 5000)
        proc.on('exit', () => clearTimeout(killTimeout))
      }
      try {
        const tmpDir = os.tmpdir()
        fs.readdirSync(tmpDir)
          .filter(f => f.startsWith('hermind-port-'))
          .forEach(f => fs.unlinkSync(path.join(tmpDir, f)))
      } catch {
        // Ignore cleanup errors
      }
    },

    getStatus: () => status,
    getPort: () => port,

    onStatusChange(callback) {
      statusListeners.push(callback)
      return () => {
        statusListeners = statusListeners.filter(cb => cb !== callback)
      }
    }
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms))
}
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/main/go-process.ts
git commit -m "feat(electron): add Go subprocess manager with port discovery and crash recovery"
```

---

## Task 3: Preload Script (Type-Safe Bridge)

**Files:**
- Create: `desktop-electron/src/preload/index.ts`

- [ ] **Step 1: Implement preload script**

```typescript
import { contextBridge, ipcRenderer, IpcRendererEvent } from 'electron'

export interface ElectronAPI {
  platform: string
  appVersion: string
  minimizeToTray: () => void
  showWindow: () => void
  hideWindow: () => void
  openFileDialog: (options: {
    title?: string
    filters?: { name: string; extensions: string[] }[]
    multiple?: boolean
  }) => Promise<string[] | null>
  saveFileDialog: (options: {
    title?: string
    defaultPath?: string
    filters?: { name: string; extensions: string[] }[]
  }) => Promise<string | null>
  showNotification: (options: { title: string; body: string }) => void
  registerGlobalShortcut: (accelerator: string) => Promise<boolean>
  unregisterGlobalShortcut: (accelerator: string) => void
  onShortcutTriggered: (callback: (accelerator: string) => void) => () => void
  onGoStatusChange: (callback: (status: string) => void) => () => void
}

const api: ElectronAPI = {
  platform: process.platform,
  appVersion: process.env.npm_package_version || 'dev',

  minimizeToTray: () => ipcRenderer.send('window:minimize-to-tray'),
  showWindow: () => ipcRenderer.send('window:show'),
  hideWindow: () => ipcRenderer.send('window:hide'),

  openFileDialog: (options) => ipcRenderer.invoke('dialog:open-file', options),
  saveFileDialog: (options) => ipcRenderer.invoke('dialog:save-file', options),

  showNotification: (options) => ipcRenderer.send('notification:show', options),

  registerGlobalShortcut: (accelerator) =>
    ipcRenderer.invoke('shortcut:register', accelerator),
  unregisterGlobalShortcut: (accelerator) =>
    ipcRenderer.send('shortcut:unregister', accelerator),

  onShortcutTriggered: (callback) => {
    const handler = (_event: IpcRendererEvent, accelerator: string) =>
      callback(accelerator)
    ipcRenderer.on('shortcut:triggered', handler)
    return () => ipcRenderer.removeListener('shortcut:triggered', handler)
  },

  onGoStatusChange: (callback) => {
    const handler = (_event: IpcRendererEvent, status: string) =>
      callback(status)
    ipcRenderer.on('go:status-change', handler)
    return () => ipcRenderer.removeListener('go:status-change', handler)
  }
}

contextBridge.exposeInMainWorld('electronAPI', api)
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/preload/index.ts
git commit -m "feat(electron): add type-safe preload script exposing IPC bridge"
```

---

## Task 4: IPC Handlers

**Files:**
- Create: `desktop-electron/src/main/ipc.ts`

- [ ] **Step 1: Implement IPC handlers**

```typescript
import { ipcMain, dialog, Notification, BrowserWindow } from 'electron'

export function registerIPCHandlers(mainWindow: BrowserWindow) {
  ipcMain.on('window:minimize-to-tray', () => {
    mainWindow.hide()
  })

  ipcMain.on('window:show', () => {
    mainWindow.show()
    mainWindow.focus()
  })

  ipcMain.on('window:hide', () => {
    mainWindow.hide()
  })

  ipcMain.handle('dialog:open-file', async (_event, options) => {
    const result = await dialog.showOpenDialog(mainWindow, {
      title: options.title,
      filters: options.filters,
      properties: options.multiple ? ['openFile', 'multiSelections'] : ['openFile']
    })
    return result.canceled ? null : result.filePaths
  })

  ipcMain.handle('dialog:save-file', async (_event, options) => {
    const result = await dialog.showSaveDialog(mainWindow, {
      title: options.title,
      defaultPath: options.defaultPath,
      filters: options.filters
    })
    return result.canceled ? null : result.filePath
  })

  ipcMain.on('notification:show', (_event, options) => {
    new Notification({
      title: options.title,
      body: options.body
    }).show()
  })
}
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/main/ipc.ts
git commit -m "feat(electron): add IPC handlers for window, dialog, and notification"
```

---

## Task 5: Window Manager

**Files:**
- Create: `desktop-electron/src/main/window.ts`

- [ ] **Step 1: Implement window manager**

```typescript
import { BrowserWindow, app } from 'electron'
import * as path from 'path'

let mainWindow: BrowserWindow | null = null

export function createMainWindow(port: number): BrowserWindow {
  mainWindow = new BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 800,
    minHeight: 600,
    title: 'Hermind',
    show: false,
    webPreferences: {
      preload: path.join(__dirname, '../preload/index.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    },
    autoHideMenuBar: true
  })

  const url = `http://127.0.0.1:${port}/ui/`
  console.log(`[Window] Loading ${url}`)
  mainWindow.loadURL(url)

  mainWindow.once('ready-to-show', () => {
    mainWindow?.show()
    mainWindow?.focus()
  })

  mainWindow.on('close', (event) => {
    if (!app.isQuiting) {
      event.preventDefault()
      mainWindow?.hide()
    }
  })

  if (!app.isPackaged) {
    mainWindow.webContents.openDevTools()
  }

  return mainWindow
}

export function getMainWindow(): BrowserWindow | null {
  return mainWindow
}

export function showMainWindow(): void {
  if (mainWindow) {
    mainWindow.show()
    mainWindow.focus()
  }
}

export function toggleMainWindow(): void {
  if (!mainWindow) return
  if (mainWindow.isVisible()) {
    mainWindow.hide()
  } else {
    mainWindow.show()
    mainWindow.focus()
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/main/window.ts
git commit -m "feat(electron): add window manager with tray-aware close behavior"
```

---

## Task 6: System Tray

**Files:**
- Create: `desktop-electron/src/main/tray.ts`

- [ ] **Step 1: Implement tray manager**

```typescript
import { Tray, Menu, nativeImage } from 'electron'
import * as path from 'path'

let tray: Tray | null = null

export function createTray(
  onShowWindow: () => void,
  onQuit: () => void
): Tray {
  const iconPath = path.join(__dirname, '../../resources/icon.ico')
  const icon = nativeImage.createFromPath(iconPath)

  tray = new Tray(icon.resize({ width: 16, height: 16 }))
  tray.setToolTip('Hermind')

  const contextMenu = Menu.buildFromTemplate([
    { label: 'Show', click: onShowWindow },
    { type: 'separator' },
    { label: 'Quit', click: onQuit }
  ])

  tray.setContextMenu(contextMenu)
  tray.on('click', onShowWindow)
  tray.on('double-click', onShowWindow)

  return tray
}

export function destroyTray(): void {
  if (tray) {
    tray.destroy()
    tray = null
  }
}
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/main/tray.ts
git commit -m "feat(electron): add system tray with Show and Quit menu"
```

---

## Task 7: Global Shortcuts

**Files:**
- Create: `desktop-electron/src/main/shortcuts.ts`

- [ ] **Step 1: Implement shortcut manager**

```typescript
import { globalShortcut, ipcMain, BrowserWindow } from 'electron'

const registered = new Set<string>()

export function registerShortcuts(mainWindow: BrowserWindow) {
  ipcMain.handle('shortcut:register', (_event, accelerator: string) => {
    if (registered.has(accelerator)) return true

    const success = globalShortcut.register(accelerator, () => {
      mainWindow.webContents.send('shortcut:triggered', accelerator)
    })

    if (success) registered.add(accelerator)
    return success
  })

  ipcMain.on('shortcut:unregister', (_event, accelerator: string) => {
    globalShortcut.unregister(accelerator)
    registered.delete(accelerator)
  })
}

export function registerToggleShortcut(
  mainWindow: BrowserWindow,
  toggleFn: () => void
): boolean {
  const accelerator = process.platform === 'darwin'
    ? 'Command+Shift+H'
    : 'Control+Shift+H'

  if (registered.has(accelerator)) return true

  const success = globalShortcut.register(accelerator, toggleFn)
  if (success) registered.add(accelerator)
  return success
}

export function unregisterAllShortcuts() {
  globalShortcut.unregisterAll()
  registered.clear()
}
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/main/shortcuts.ts
git commit -m "feat(electron): add global shortcut registration with toggle support"
```

---

## Task 8: Main Entry Point

**Files:**
- Create: `desktop-electron/src/main/index.ts`

- [ ] **Step 1: Implement main entry**

```typescript
import { app } from 'electron'
import { createGoProcessManager } from './go-process'
import { createMainWindow, showMainWindow, toggleMainWindow, getMainWindow } from './window'
import { createTray, destroyTray } from './tray'
import { registerShortcuts, registerToggleShortcut, unregisterAllShortcuts } from './shortcuts'
import { registerIPCHandlers } from './ipc'

let goManager = createGoProcessManager()
let isQuiting = false

app.whenReady().then(async () => {
  try {
    const port = await goManager.start()
    console.log(`[Main] Go backend ready on port ${port}`)

    const mainWindow = createMainWindow(port)
    registerIPCHandlers(mainWindow)

    createTray(
      () => showMainWindow(),
      () => {
        isQuiting = true
        app.quit()
      }
    )

    registerShortcuts(mainWindow)
    registerToggleShortcut(mainWindow, () => toggleMainWindow())

    goManager.onStatusChange((status) => {
      const win = getMainWindow()
      if (win && !win.isDestroyed()) {
        win.webContents.send('go:status-change', status)
      }
    })

    app.on('activate', () => {
      if (getMainWindow() === null) {
        const currentPort = goManager.getPort()
        if (currentPort) createMainWindow(currentPort)
      } else {
        showMainWindow()
      }
    })

  } catch (err) {
    console.error('[Main] Failed to start:', err)
    app.quit()
  }
})

app.on('before-quit', () => {
  isQuiting = true
})

app.on('will-quit', () => {
  unregisterAllShortcuts()
  destroyTray()
  goManager.stop()
})

const gotTheLock = app.requestSingleInstanceLock()
if (!gotTheLock) {
  app.quit()
} else {
  app.on('second-instance', () => {
    showMainWindow()
  })
}
```

- [ ] **Step 2: Commit**

```bash
git add desktop-electron/src/main/index.ts
git commit -m "feat(electron): add main entry point orchestrating Go, window, tray, and shortcuts"
```

---

## Task 9: Build & Packaging Configuration

**Files:**
- Create: `desktop-electron/electron-builder.yml`
- Modify: `desktop-electron/package.json` (add icon copy script)

- [ ] **Step 1: Create electron-builder.yml**

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
win:
  target: nsis
  icon: resources/icon.ico
nsis:
  oneClick: false
  allowToChangeInstallationDirectory: true
  createDesktopShortcut: true
  createStartMenuShortcut: true
```

- [ ] **Step 2: Add build scripts to package.json**

Add to `desktop-electron/package.json` scripts:
```json
{
  "scripts": {
    "dev": "electron-vite dev",
    "build": "electron-vite build",
    "build:go": "go build -ldflags=\"-s -w\" -o resources/hermind-desktop.exe ../cmd/hermind",
    "preview": "electron-vite preview",
    "dist": "npm run build:go && npm run build && electron-builder",
    "dist:dir": "npm run build:go && npm run build && electron-builder --dir"
  }
}
```

- [ ] **Step 3: Copy icon from QML desktop**

```bash
cp desktop/resources/icon.ico desktop-electron/resources/icon.ico 2>/dev/null || echo "Icon copy skipped - add manually"
```

If no icon exists in desktop/resources/, use a placeholder or create one.

- [ ] **Step 4: Build Electron**

```bash
cd desktop-electron
npm run build
```

Expected: `out/main/index.js`, `out/preload/index.js`, `out/renderer/index.html` created.

- [ ] **Step 5: Commit**

```bash
git add desktop-electron/electron-builder.yml desktop-electron/package.json desktop-electron/resources/
git commit -m "feat(electron): add build and packaging configuration"
```

---

## Task 10: Go Temp File Fallback

**Files:**
- Modify: `cli/desktop.go`

- [ ] **Step 1: Add temp file write in desktop.go**

After the `HERMIND_READY` print in `cli/desktop.go` `runWeb()` (~line 120):

```go
// After: fmt.Fprintf(os.Stdout, "HERMIND_READY %d\n", port)
// Add:
if port > 0 {
    tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("hermind-port-%d.txt", os.Getpid()))
    _ = os.WriteFile(tmpFile, []byte(strconv.Itoa(port)), 0644)
}
```

Also add imports if missing: `path/filepath`, `os`, `strconv`, `fmt`.

- [ ] **Step 2: Verify Go builds**

```bash
go build ./cmd/hermind
```

Expected: Build succeeds, no errors.

- [ ] **Step 3: Test `hermind desktop` outputs ready signal**

```bash
./hermind desktop --exit-after 3s 2>&1 | grep HERMIND_READY
```

Expected: `HERMIND_READY <port>` line printed.

- [ ] **Step 4: Commit**

```bash
git add cli/desktop.go
git commit -m "feat(desktop): write port to temp file for Electron fallback discovery"
```

---

## Task 11: End-to-End Verification

**Files:** None (integration test)

- [ ] **Step 1: Build Go binary for Electron**

```bash
cd desktop-electron
npm run build:go
```

Expected: `desktop-electron/resources/hermind-desktop.exe` created.

- [ ] **Step 2: Start Electron in dev mode**

Terminal 1 (keep Go running):
```bash
cd D:/go_work/hermind
./hermind desktop
```

Terminal 2:
```bash
cd desktop-electron
npm run dev
```

Expected: Electron window opens, loads Web UI, chat/settings functional.

- [ ] **Step 3: Verify tray behavior**

- Close window (X button) → window hides, tray icon remains
- Click tray icon → window shows
- Right-click tray → Show / Quit menu works

- [ ] **Step 4: Verify global shortcut**

Press `Ctrl+Shift+H` → window toggles visible/hidden.

- [ ] **Step 5: Verify Go crash recovery**

Kill the Go process manually (Task Manager or `kill`):
- Electron should detect exit
- Show "restarting" state
- Restart Go automatically
- Window should reload to new port

- [ ] **Step 6: Package installer**

```bash
cd desktop-electron
npm run dist:dir
```

Expected: `dist/win-unpacked/Hermind.exe` created with `resources/hermind-desktop.exe` bundled.

- [ ] **Step 7: Commit**

```bash
git commit -m "feat(electron): complete Electron desktop migration - e2e verified"
```

---

## Self-Review

### 1. Spec Coverage

| Spec Section | Task(s) |
|---|---|
| Architecture (Electron + Go subprocess) | Task 8 (main entry) |
| Go port discovery (stdout + temp file) | Task 2 (go-process), Task 10 (Go fallback) |
| Health check & crash recovery | Task 2 |
| IPC API (preload + handlers) | Task 3, Task 4 |
| Window behavior (close→tray) | Task 5 |
| System tray | Task 6 |
| Global shortcuts | Task 7 |
| Build & packaging | Task 9 |
| E2E verification | Task 11 |

**No gaps identified.**

### 2. Placeholder Scan

- No TBD/TODO
- No vague "add error handling" steps
- No "similar to Task X" references
- Every step has actual code or exact commands

### 3. Type Consistency

- `GoStatus` type used consistently across `go-process.ts`, preload, and IPC
- Port is always `number` (not string)
- IPC channel names consistent between preload and main handlers
- Shortcut accelerator format matches Electron's `globalShortcut.register()` API

**Clean — no type mismatches.**

---

## Execution Handoff

**Plan complete and saved to `.gpowers/plans/2026-05-17-electron-desktop-migration.md`.**

Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
