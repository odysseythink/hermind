"use strict";
const electron = require("electron");
const child_process = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");
function _interopNamespaceDefault(e) {
  const n = Object.create(null, { [Symbol.toStringTag]: { value: "Module" } });
  if (e) {
    for (const k in e) {
      if (k !== "default") {
        const d = Object.getOwnPropertyDescriptor(e, k);
        Object.defineProperty(n, k, d.get ? d : {
          enumerable: true,
          get: () => e[k]
        });
      }
    }
  }
  n.default = e;
  return Object.freeze(n);
}
const path__namespace = /* @__PURE__ */ _interopNamespaceDefault(path);
const MAX_RETRIES = 5;
const RETRY_DELAY_MS = 2e3;
const HEALTH_POLL_INTERVAL_MS = 500;
const HEALTH_POLL_MAX_ATTEMPTS = 60;
const PORT_DISCOVERY_TIMEOUT_MS = 1e4;
const SIGKILL_DELAY_MS = 5e3;
const READY_PATTERN = /HERMIND_READY\s+(\d+)/;
function createGoProcessManager() {
  let status = "starting";
  let port = null;
  let child = null;
  let retryCount = 0;
  let stopRequested = false;
  let portTimeout = null;
  let sigkillTimeout = null;
  const statusCallbacks = /* @__PURE__ */ new Set();
  let startResolve = null;
  let startReject = null;
  function setStatus(newStatus) {
    if (status !== newStatus) {
      status = newStatus;
      for (const cb of statusCallbacks) {
        try {
          cb(status);
        } catch {
        }
      }
    }
  }
  function getBinaryInfo() {
    if (electron.app.isPackaged) {
      const exePath = path.join(process.resourcesPath, "hermind-desktop.exe");
      return { command: exePath, args: ["desktop"], cwd: process.cwd() };
    }
    const devExePath = path.join(__dirname, "..", "..", "resources", "hermind-desktop.exe");
    if (fs.existsSync(devExePath)) {
      return { command: devExePath, args: ["desktop"], cwd: process.cwd() };
    }
    const goCwd = path.join(__dirname, "..", "..", "..");
    return { command: "go", args: ["run", "./cmd/hermind", "desktop"], cwd: goCwd };
  }
  function getTempFilePath(pid) {
    return path.join(os.tmpdir(), `hermind-port-${pid}.txt`);
  }
  function cleanupTempFiles() {
    try {
      const tmpDir = os.tmpdir();
      const files = fs.readdirSync(tmpDir);
      for (const file of files) {
        if (file.startsWith("hermind-port-")) {
          fs.unlinkSync(path.join(tmpDir, file));
        }
      }
    } catch {
    }
  }
  function tryReadPortFromTempFile(pid) {
    try {
      const tempFile = getTempFilePath(pid);
      if (fs.existsSync(tempFile)) {
        const content = fs.readFileSync(tempFile, "utf-8").trim();
        const parsed = parseInt(content, 10);
        if (!isNaN(parsed) && parsed > 0) {
          return parsed;
        }
      }
    } catch {
    }
    return null;
  }
  async function pollHealth(targetPort) {
    for (let i = 0; i < HEALTH_POLL_MAX_ATTEMPTS; i++) {
      if (stopRequested) return false;
      try {
        const response = await fetch(`http://127.0.0.1:${targetPort}/api/health`);
        if (response.status === 200) {
          return true;
        }
      } catch {
      }
      await new Promise((resolve) => setTimeout(resolve, HEALTH_POLL_INTERVAL_MS));
    }
    return false;
  }
  async function handlePortDiscovery(discoveredPort, pid) {
    if (port !== null) return;
    port = discoveredPort;
    if (portTimeout) {
      clearTimeout(portTimeout);
      portTimeout = null;
    }
    const healthy = await pollHealth(port);
    if (!healthy) {
      killChild();
      if (startReject) {
        startReject(new Error(`Health check failed for port ${port}`));
        startResolve = null;
        startReject = null;
      }
      return;
    }
    setStatus("running");
    retryCount = 0;
    if (startResolve) {
      startResolve(port);
      startResolve = null;
      startReject = null;
    }
  }
  function killChild() {
    if (!child) return;
    if (sigkillTimeout) {
      clearTimeout(sigkillTimeout);
      sigkillTimeout = null;
    }
    child.kill("SIGTERM");
    sigkillTimeout = setTimeout(() => {
      if (child && !child.killed) {
        child.kill("SIGKILL");
      }
    }, SIGKILL_DELAY_MS);
  }
  function spawnProcess() {
    if (stopRequested) return;
    const { command, args, cwd } = getBinaryInfo();
    setStatus(retryCount > 0 ? "restarting" : "starting");
    const proc = child_process.spawn(command, args, {
      cwd,
      stdio: ["ignore", "pipe", "pipe"],
      windowsHide: true
    });
    child = proc;
    let stdoutBuffer = "";
    proc.stdout?.on("data", (data) => {
      const chunk = data.toString();
      stdoutBuffer += chunk;
      const match = READY_PATTERN.exec(stdoutBuffer);
      if (match) {
        const discoveredPort = parseInt(match[1], 10);
        if (!isNaN(discoveredPort)) {
          handlePortDiscovery(discoveredPort, proc.pid ?? 0);
        }
      }
    });
    proc.stderr?.on("data", (data) => {
      console.error(`[Go stderr] ${data.toString().trim()}`);
    });
    portTimeout = setTimeout(() => {
      if (port === null && proc.pid) {
        const tempPort = tryReadPortFromTempFile(proc.pid);
        if (tempPort !== null) {
          handlePortDiscovery(tempPort, proc.pid ?? 0);
        }
      }
    }, PORT_DISCOVERY_TIMEOUT_MS);
    proc.on("error", (err) => {
      setStatus("error");
      if (portTimeout) {
        clearTimeout(portTimeout);
        portTimeout = null;
      }
      if (startReject) {
        startReject(err);
        startResolve = null;
        startReject = null;
      }
    });
    proc.on("exit", () => {
      child = null;
      if (portTimeout) {
        clearTimeout(portTimeout);
        portTimeout = null;
      }
      if (stopRequested) {
        cleanupTempFiles();
        return;
      }
      if (port !== null) {
        port = null;
        setStatus("error");
      }
      if (retryCount < MAX_RETRIES) {
        retryCount++;
        setTimeout(() => {
          if (!stopRequested) {
            spawnProcess();
          }
        }, RETRY_DELAY_MS);
      } else {
        setStatus("error");
        if (startReject) {
          startReject(new Error("Go process failed to start after maximum retries"));
          startResolve = null;
          startReject = null;
        }
      }
    });
  }
  return {
    start() {
      if (port !== null) {
        return Promise.resolve(port);
      }
      if (startResolve) {
        return new Promise((resolve, reject) => {
          const check = setInterval(() => {
            if (port !== null) {
              clearInterval(check);
              resolve(port);
            } else if (status === "error" && !startResolve) {
              clearInterval(check);
              reject(new Error("Go process failed to start"));
            }
          }, 100);
        });
      }
      stopRequested = false;
      return new Promise((resolve, reject) => {
        startResolve = resolve;
        startReject = reject;
        spawnProcess();
      });
    },
    stop() {
      stopRequested = true;
      retryCount = MAX_RETRIES;
      port = null;
      if (portTimeout) {
        clearTimeout(portTimeout);
        portTimeout = null;
      }
      killChild();
      if (startReject) {
        startReject(new Error("Go process stopped"));
        startResolve = null;
        startReject = null;
      }
      cleanupTempFiles();
    },
    getStatus() {
      return status;
    },
    getPort() {
      return port;
    },
    onStatusChange(callback) {
      statusCallbacks.add(callback);
      return () => {
        statusCallbacks.delete(callback);
      };
    }
  };
}
let mainWindow = null;
let isQuiting = false;
electron.app.on("before-quit", () => {
  isQuiting = true;
});
function createMainWindow(port) {
  mainWindow = new electron.BrowserWindow({
    width: 1400,
    height: 900,
    minWidth: 800,
    minHeight: 600,
    title: "Hermind",
    show: false,
    webPreferences: {
      preload: path__namespace.join(__dirname, "../preload/index.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true
    },
    autoHideMenuBar: true
  });
  const url = `http://127.0.0.1:${port}/ui/`;
  console.log(`[Window] Loading ${url}`);
  mainWindow.loadURL(url);
  mainWindow.once("ready-to-show", () => {
    mainWindow?.show();
    mainWindow?.focus();
  });
  mainWindow.on("close", (event) => {
    if (!isQuiting) {
      event.preventDefault();
      mainWindow?.hide();
    }
  });
  if (!electron.app.isPackaged) {
    mainWindow.webContents.openDevTools();
  }
  return mainWindow;
}
function getMainWindow() {
  return mainWindow;
}
function showMainWindow() {
  if (mainWindow) {
    mainWindow.show();
    mainWindow.focus();
  }
}
function toggleMainWindow() {
  if (!mainWindow) return;
  if (mainWindow.isVisible()) {
    mainWindow.hide();
  } else {
    mainWindow.show();
    mainWindow.focus();
  }
}
let tray = null;
function createTray(onShowWindow, onQuit) {
  const iconPath = path__namespace.join(__dirname, "../../resources/icon.ico");
  let icon = electron.nativeImage.createFromPath(iconPath);
  if (icon.isEmpty()) {
    icon = electron.nativeImage.createEmpty();
  }
  tray = new electron.Tray(icon);
  tray.setToolTip("Hermind");
  const contextMenu = electron.Menu.buildFromTemplate([
    { label: "Show", click: onShowWindow },
    { type: "separator" },
    { label: "Quit", click: onQuit }
  ]);
  tray.setContextMenu(contextMenu);
  tray.on("click", onShowWindow);
  tray.on("double-click", onShowWindow);
  return tray;
}
function destroyTray() {
  if (tray) {
    tray.destroy();
    tray = null;
  }
}
const registered = /* @__PURE__ */ new Set();
function registerShortcuts(mainWindow2) {
  electron.ipcMain.handle("shortcut:register", (_event, accelerator) => {
    if (registered.has(accelerator)) return true;
    const success = electron.globalShortcut.register(accelerator, () => {
      mainWindow2.webContents.send("shortcut:triggered", accelerator);
    });
    if (success) registered.add(accelerator);
    return success;
  });
  electron.ipcMain.on("shortcut:unregister", (_event, accelerator) => {
    electron.globalShortcut.unregister(accelerator);
    registered.delete(accelerator);
  });
}
function registerToggleShortcut(mainWindow2, toggleFn) {
  const accelerator = process.platform === "darwin" ? "Command+Shift+H" : "Control+Shift+H";
  if (registered.has(accelerator)) return true;
  const success = electron.globalShortcut.register(accelerator, toggleFn);
  if (success) registered.add(accelerator);
  return success;
}
function unregisterAllShortcuts() {
  electron.globalShortcut.unregisterAll();
  registered.clear();
}
function registerIPCHandlers(mainWindow2) {
  electron.ipcMain.on("window:minimize-to-tray", () => {
    mainWindow2.hide();
  });
  electron.ipcMain.on("window:show", () => {
    mainWindow2.show();
    mainWindow2.focus();
  });
  electron.ipcMain.on("window:hide", () => {
    mainWindow2.hide();
  });
  electron.ipcMain.handle("dialog:open-file", async (_event, options) => {
    const result = await electron.dialog.showOpenDialog(mainWindow2, {
      title: options.title,
      filters: options.filters,
      properties: options.multiple ? ["openFile", "multiSelections"] : ["openFile"]
    });
    return result.canceled ? null : result.filePaths;
  });
  electron.ipcMain.handle("dialog:save-file", async (_event, options) => {
    const result = await electron.dialog.showSaveDialog(mainWindow2, {
      title: options.title,
      defaultPath: options.defaultPath,
      filters: options.filters
    });
    return result.canceled ? null : result.filePath;
  });
  electron.ipcMain.on("notification:show", (_event, options) => {
    new electron.Notification({
      title: options.title,
      body: options.body
    }).show();
  });
}
let goManager = createGoProcessManager();
electron.app.whenReady().then(async () => {
  try {
    const port = await goManager.start();
    console.log(`[Main] Go backend ready on port ${port}`);
    const mainWindow2 = createMainWindow(port);
    registerIPCHandlers(mainWindow2);
    createTray(
      () => showMainWindow(),
      () => {
        electron.app.quit();
      }
    );
    registerShortcuts(mainWindow2);
    registerToggleShortcut(mainWindow2, () => toggleMainWindow());
    goManager.onStatusChange((status) => {
      const win = getMainWindow();
      if (win && !win.isDestroyed()) {
        win.webContents.send("go:status-change", status);
      }
    });
    electron.app.on("activate", () => {
      if (getMainWindow() === null) {
        const currentPort = goManager.getPort();
        if (currentPort) createMainWindow(currentPort);
      } else {
        showMainWindow();
      }
    });
  } catch (err) {
    console.error("[Main] Failed to start:", err);
    electron.app.quit();
  }
});
electron.app.on("will-quit", () => {
  unregisterAllShortcuts();
  destroyTray();
  goManager.stop();
});
const gotTheLock = electron.app.requestSingleInstanceLock();
if (!gotTheLock) {
  electron.app.quit();
} else {
  electron.app.on("second-instance", () => {
    showMainWindow();
  });
}
