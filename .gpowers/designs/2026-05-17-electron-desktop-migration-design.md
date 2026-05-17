# Electron Desktop Migration Design

**Date**: 2026-05-17  
**Status**: Approved, awaiting implementation plan  
**Context**: QML desktop development is error-prone under AI assistance. Migrate to Electron wrapping the existing Web UI.

---

## 1. Problem Statement

The current desktop application uses Qt6 QML + Go backend via CGO. Development has been plagued by:
- Runtime QML errors that cascade ("Non-existent attached object", missing imports)
- Weak type system in QML causing silent failures
- AI tools struggle with Qt/QML debugging and state management
- Glassmorphism redesign required ~50 file changes with repeated runtime errors

**Goal**: Replace QML desktop with Electron + existing Web UI, reusing the Go HTTP API.

---

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Electron Main Process                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐ │
│  │   Window    │  │    Tray     │  │   Go Process Mgr    │ │
│  │   Manager   │  │   Manager   │  │  (spawn / restart)  │ │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘ │
│         │                │                      │            │
│         └────────────────┼──────────────────────┘            │
│                          ▼                                  │
│              ┌─────────────────────┐                        │
│              │   IPC Handlers      │                        │
│              │ (expose to renderer)│                        │
│              └──────────┬──────────┘                        │
└─────────────────────────┼───────────────────────────────────┘
                          │ contextBridge
┌─────────────────────────┼───────────────────────────────────┐
│              Electron Renderer Process                       │
│  ┌──────────────────────┴─────────────────────┐             │
│  │          Preload Script                    │             │
│  │   window.electronAPI = { ... }             │             │
│  └──────────────────────┬─────────────────────┘             │
│                         │                                    │
│  ┌──────────────────────┴─────────────────────┐             │
│  │          Existing Web UI (React/Vite)      │             │
│  │  Loaded from http://localhost:<go-port>/ui/ │             │
│  │  - Chat, Settings, Theme, Language...      │             │
│  └────────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────┘
                          │ HTTP / SSE
┌─────────────────────────┼───────────────────────────────────┐
│              Go HTTP Server (child process)                  │
│  ┌──────────────────────┴─────────────────────┐             │
│  │  REST API + SSE streaming + Web UI static  │             │
│  │  (chi router, existing api/ package)       │             │
│  └────────────────────────────────────────────┘             │
└─────────────────────────────────────────────────────────────┘
```

**Principles**:
- Electron does NOT handle business logic. It only: starts Go, manages window, bridges native capabilities.
- Web UI calls Go HTTP API via `fetch()`, identical to browser behavior.
- Native capabilities (tray, shortcuts, file dialogs) via `window.electronAPI`.
- Web UI degrades gracefully when `window.electronAPI` is absent (browser mode).

---

## 3. Go Subprocess Lifecycle

### 3.1 Port Assignment

Go auto-binds to an available port. Electron discovers it via stdout:

```
Go startup (desktop mode):
  $ hermind-desktop --desktop-mode --port=0
  
  # On ready, prints to stdout:
  HERMIND_READY 49231
  
  # Fallback: writes to %TEMP%/hermind-port-<pid>.txt
```

Electron reads port via:
1. Primary: stdout line matching `HERMIND_PORT=(\d+)`
2. Fallback: read temp file if stdout parsing fails within 10s

### 3.2 Startup Sequence

```
1. Electron main starts
2. Spawn Go child process
3. Wait for HERMIND_PORT on stdout (timeout: 15s)
4. Poll GET http://localhost:<port>/api/health
5. Health 200 → create BrowserWindow, load http://localhost:<port>/ui/
6. Health timeout → show error page with "Retry" button
```

### 3.3 Crash Recovery

```
Go process exits unexpectedly:
  1. Hide window, show "Restarting service..." overlay
  2. Delay 2s, respawn Go process
  3. Max 5 retries, then show fatal error page
  4. User can click "Retry" to manually restart
```

### 3.4 Shutdown

```
Electron before-quit:
  1. Send SIGTERM to Go child process
  2. Wait 5s for graceful shutdown
  3. SIGKILL if still running
  4. Clean up %TEMP%/hermind-port-*.txt
```

---

## 4. IPC API Design

### 4.1 Preload Script API

```typescript
interface ElectronAPI {
  // Platform info
  platform: 'win32' | 'darwin' | 'linux';
  appVersion: string;

  // Window control
  minimizeToTray: () => void;
  showWindow: () => void;
  hideWindow: () => void;
  isWindowVisible: () => boolean;

  // System tray
  setTrayTooltip: (tooltip: string) => void;
  setTrayIcon: (iconPath: string) => void;

  // Global shortcuts
  registerGlobalShortcut: (
    accelerator: string,
    callbackId: string
  ) => Promise<boolean>;
  unregisterGlobalShortcut: (accelerator: string) => void;
  // Callbacks delivered via ipcRenderer.on('shortcut-triggered', ...)

  // File dialogs
  openFileDialog: (options: {
    title?: string;
    filters?: { name: string; extensions: string[] }[];
    multiple?: boolean;
  }) => Promise<string[] | null>;

  saveFileDialog: (options: {
    title?: string;
    defaultPath?: string;
    filters?: { name: string; extensions: string[] }[];
  }) => Promise<string | null>;

  // Notifications
  showNotification: (options: {
    title: string;
    body: string;
    icon?: string;
  }) => void;

  // Go process status
  onGoStatusChange: (
    callback: (status: 'starting' | 'running' | 'restarting' | 'error') => void
  ) => () => void;
}
```

### 4.2 Web UI Bridge Layer

Web UI creates a thin adapter that detects Electron and falls back to browser APIs:

```typescript
// In Web UI: src/electron-bridge.ts
const api = (window as any).electronAPI as ElectronAPI | undefined;
export const isElectron = !!api;

export const electron = {
  minimizeToTray: () => api?.minimizeToTray(),
  showWindow: () => api?.showWindow(),
  
  openFile: (opts) => api 
    ? api.openFileDialog(opts)
    : fallbackFilePicker(opts),
  
  notify: (opts) => api
    ? api.showNotification(opts)
    : new Notification(opts.title, { body: opts.body }),
  
  registerShortcut: (acc, cb) => api
    ? api.registerGlobalShortcut(acc, cb)
    : Promise.resolve(false),
};
```

### 4.3 QML Feature Mapping

| QML Feature | IPC API | Notes |
|---|---|---|
| `Ctrl+Shift+H` toggle window | `registerGlobalShortcut("CommandOrControl+Shift+H", "toggle")` | Only shortcut currently registered |
| Tray icon + menu | `Tray` module in main, `setTrayTooltip`/`setTrayIcon` in preload | Show / Quit menu items |
| Tray click → show window | `tray.on('click')` → `showWindow()` | |
| Tray notification | `showNotification()` | |
| Theme (C++ singleton) | **No bridge needed** | Web UI has its own theme system |
| Language switch | **No bridge needed** | Web UI handles i18n |
| Window close → tray | Intercept `beforeunload` → `minimizeToTray()` | |

---

## 5. Project Structure

```
hermind/
├── desktop/                    # Existing QML desktop (preserved)
│   ├── src/
│   ├── qml/
│   └── CMakeLists.txt
│
├── desktop-electron/           # NEW Electron desktop
│   ├── package.json
│   ├── tsconfig.json
│   ├── electron.vite.config.ts
│   ├── electron-builder.yml
│   ├── resources/
│   │   └── icon.ico
│   ├── src/
│   │   ├── main/
│   │   │   ├── index.ts        # Entry: setup Go + create window
│   │   │   ├── go-process.ts   # Go child process management
│   │   │   ├── window.ts       # BrowserWindow creation + state
│   │   │   ├── tray.ts         # System tray setup
│   │   │   ├── shortcuts.ts    # Global shortcut registration
│   │   │   └── ipc.ts          # IPC handler definitions
│   │   ├── preload/
│   │   │   └── index.ts        # contextBridge exposure
│   │   └── renderer/
│   │       └── index.html      # Dev: Vite dev server; Prod: loaded from Go
│   └── build/                  # Build output
│
├── api/webroot/                # Existing Web UI (REUSED)
├── cmd/hermind/                # Go CLI entry
└── cmd/hermind-desktop/        # NEW Go desktop entry (HTTP server mode)
```

---

## 6. Go Backend Changes

### 6.1 Reuse Existing `hermind desktop` Command

**No new Go entry point needed.** The existing `cli/desktop.go` already does exactly what we need:

```go
// cli/desktop.go — ALREADY EXISTS
func newDesktopCmd(app *App) *cobra.Command {
    var opts webRunOptions
    c := &cobra.Command{
        Use:   "desktop",
        Short: "Start backend server for desktop client",
        RunE: func(cmd *cobra.Command, args []string) error {
            opts.NoBrowser = true
            opts.PrintReadySignal = true        // ← prints HERMIND_READY <port>
            opts.FileLogPath = desktopLogPath()
            opts.Out = cmd.OutOrStdout()
            return runWeb(cmd.Context(), app, opts)
        },
    }
    // ...
}
```

Output format:
```
hermind web listening on http://127.0.0.1:49231
instance:  /path/to/instance
open:      http://127.0.0.1:49231/
HERMIND_READY 49231
```

### 6.2 Minimal Changes to Existing Code

The existing `api/` package already serves the Web UI from `api/webroot/`. No changes needed to HTTP handlers or Web UI.

The only potential addition: write the port to a temp file as a fallback (in case stdout parsing fails). This is a small enhancement to `cli/desktop.go`.

---

## 7. Build & Packaging

### 7.1 Development Workflow

```bash
cd desktop-electron

# Terminal 1: Start Go in desktop mode
go run ../cmd/hermind-desktop --port=8080

# Terminal 2: Start Electron dev
npm run dev
# Loads http://localhost:8080/ui/ in Electron
```

### 7.2 Production Build

```bash
# 1. Build Go binary
go build -ldflags="-s -w" -o desktop-electron/resources/hermind-desktop.exe ./cmd/hermind-desktop

# 2. Build Electron
npm run build

# 3. Package installer
npm run dist
# Outputs: dist/hermind-setup-1.0.0.exe
```

### 7.3 Installer Layout

```
hermind.exe                 # Electron main executable
resources/
  ├── hermind-desktop.exe   # Go HTTP server binary
  └── icon.ico             # App icon
```

---

## 8. Window Behavior (Parity with QML)

| Behavior | QML Implementation | Electron Implementation |
|---|---|---|
| Launch | Show main window | Start Go → health check → show window |
| Close button | Minimize to tray | `event.preventDefault()` + `win.hide()` |
| Tray left-click | Show/raise window | `tray.on('click')` → `win.show()` |
| Tray right-click | Show / Quit menu | `Tray` with `Menu` |
| Tray double-click | Show window | Same as left-click |
| `Ctrl+Shift+H` | Toggle visibility | `globalShortcut.register()` |
| Quit from tray | Exit application | `app.quit()` |
| Notification | `QSystemTrayIcon::showMessage()` | `Notification` constructor |

---

## 9. Out of Scope (Phase 2)

- Auto-updater (electron-updater)
- Code signing
- macOS app notarization
- Additional global shortcuts beyond `Ctrl+Shift+H`
- Deep linking (URL protocol handlers)
- Single instance lock

---

## 10. Migration Strategy

1. **Phase 1**: Build Electron shell with Go subprocess, IPC bridge, tray, shortcuts
2. **Phase 2**: Test full feature parity (chat, settings, streaming, file dialogs)
3. **Phase 3**: Add Web UI Electron adapter (`src/electron-bridge.ts`)
4. **Phase 4**: Build production installer, internal testing
5. **Phase 5**: Mark QML desktop as deprecated, remove in future release

QML code is **preserved, not deleted**. The `desktop/` directory stays as-is until Electron version is fully validated.
