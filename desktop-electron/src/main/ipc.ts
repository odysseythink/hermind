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
