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
