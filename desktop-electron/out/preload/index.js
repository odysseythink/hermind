"use strict";
const electron = require("electron");
const api = {
  platform: process.platform,
  appVersion: process.env.npm_package_version || "dev",
  minimizeToTray: () => electron.ipcRenderer.send("window:minimize-to-tray"),
  showWindow: () => electron.ipcRenderer.send("window:show"),
  hideWindow: () => electron.ipcRenderer.send("window:hide"),
  openFileDialog: (options) => electron.ipcRenderer.invoke("dialog:open-file", options),
  saveFileDialog: (options) => electron.ipcRenderer.invoke("dialog:save-file", options),
  showNotification: (options) => electron.ipcRenderer.send("notification:show", options),
  registerGlobalShortcut: (accelerator) => electron.ipcRenderer.invoke("shortcut:register", accelerator),
  unregisterGlobalShortcut: (accelerator) => electron.ipcRenderer.send("shortcut:unregister", accelerator),
  onShortcutTriggered: (callback) => {
    const handler = (_event, accelerator) => callback(accelerator);
    electron.ipcRenderer.on("shortcut:triggered", handler);
    return () => electron.ipcRenderer.removeListener("shortcut:triggered", handler);
  },
  onGoStatusChange: (callback) => {
    const handler = (_event, status) => callback(status);
    electron.ipcRenderer.on("go:status-change", handler);
    return () => electron.ipcRenderer.removeListener("go:status-change", handler);
  }
};
electron.contextBridge.exposeInMainWorld("electronAPI", api);
