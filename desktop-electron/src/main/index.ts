import { app } from 'electron'
import { createGoProcessManager } from './go-process'
import { createMainWindow, showMainWindow, toggleMainWindow, getMainWindow } from './window'
import { createTray, destroyTray } from './tray'
import { registerShortcuts, registerToggleShortcut, unregisterAllShortcuts } from './shortcuts'
import { registerIPCHandlers } from './ipc'
import * as fs from 'fs'
import * as path from 'path'
import * as os from 'os'

const logPath = path.join(app.getPath('userData'), 'electron-startup.log')
function log(msg: string) {
  const line = `[${new Date().toISOString()}] ${msg}\n`
  console.log(line.trim())
  try { fs.appendFileSync(logPath, line) } catch {}
}

log('=== Main process starting ===')
log('appPath: ' + app.getAppPath())
log('resourcesPath: ' + process.resourcesPath)

// If the installer/uninstaller left a backup of .hermind in TEMP, restore it.
function restoreHermindData() {
  if (!app.isPackaged) return
  const installDir = path.dirname(process.resourcesPath)
  const hermindDir = path.join(installDir, '.hermind')
  const backupDir = path.join(os.tmpdir(), 'hermind-data-backup')
  if (fs.existsSync(backupDir) && !fs.existsSync(hermindDir)) {
    try {
      fs.cpSync(backupDir, hermindDir, { recursive: true, force: true })
      fs.rmSync(backupDir, { recursive: true, force: true })
      log('Restored .hermind from temp backup')
    } catch (err) {
      log('Failed to restore .hermind: ' + (err as Error).message)
    }
  }
}
restoreHermindData()

let goManager = createGoProcessManager()

app.whenReady().then(async () => {
  log('app.whenReady fired')
  try {
    const port = await goManager.start()
    log(`Go backend ready on port ${port}`)

    const mainWindow = createMainWindow(port)
    registerIPCHandlers(mainWindow)

    createTray(
      () => showMainWindow(),
      () => {
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
    log('FAILED TO START: ' + (err as Error).message)
    console.error('[Main] Failed to start:', err)
    app.quit()
  }
})

app.on('will-quit', () => {
  log('will-quit')
  unregisterAllShortcuts()
  destroyTray()
  goManager.stop()
})

app.on('window-all-closed', () => {
  log('window-all-closed')
})

const gotTheLock = app.requestSingleInstanceLock()
if (!gotTheLock) {
  app.quit()
} else {
  app.on('second-instance', () => {
    showMainWindow()
  })
}
