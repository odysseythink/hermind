import { Tray, Menu, nativeImage } from 'electron'
import * as path from 'path'

let tray: Tray | null = null

export function createTray(
  onShowWindow: () => void,
  onQuit: () => void
): Tray {
  const iconPath = path.join(__dirname, '../../resources/icon.ico')
  let icon = nativeImage.createFromPath(iconPath)
  if (icon.isEmpty()) {
    icon = nativeImage.createEmpty()
  }

  tray = new Tray(icon)
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
