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
