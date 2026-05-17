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
