import { spawn, ChildProcess } from 'child_process'
import path from 'path'
import fs from 'fs'
import os from 'os'
import { app } from 'electron'

export type GoStatus = 'starting' | 'running' | 'restarting' | 'error'

export interface GoProcessManager {
  start(): Promise<number>
  stop(): void
  getStatus(): GoStatus
  getPort(): number | null
  onStatusChange(callback: (status: GoStatus) => void): () => void
}

const MAX_RETRIES = 5
const RETRY_DELAY_MS = 2000
const HEALTH_POLL_INTERVAL_MS = 500
const HEALTH_POLL_MAX_ATTEMPTS = 60
const PORT_DISCOVERY_TIMEOUT_MS = 10000
const SIGKILL_DELAY_MS = 5000
const READY_PATTERN = /HERMIND_READY\s+(\d+)/

export function createGoProcessManager(): GoProcessManager {
  let status: GoStatus = 'starting'
  let port: number | null = null
  let child: ChildProcess | null = null
  let retryCount = 0
  let stopRequested = false
  let portTimeout: ReturnType<typeof setTimeout> | null = null
  let sigkillTimeout: ReturnType<typeof setTimeout> | null = null
  const statusCallbacks = new Set<(status: GoStatus) => void>()
  let startResolve: ((value: number) => void) | null = null
  let startReject: ((reason: Error) => void) | null = null

  function setStatus(newStatus: GoStatus): void {
    if (status !== newStatus) {
      status = newStatus
      for (const cb of statusCallbacks) {
        try {
          cb(status)
        } catch {
          // ignore callback errors
        }
      }
    }
  }

  function getBinaryInfo(): { command: string; args: string[]; cwd: string } {
    if (app.isPackaged) {
      const exePath = path.join(process.resourcesPath, 'hermind-desktop.exe')
      return { command: exePath, args: ['desktop'], cwd: process.cwd() }
    }

    const devExePath = path.join(__dirname, '..', '..', 'resources', 'hermind-desktop.exe')
    if (fs.existsSync(devExePath)) {
      return { command: devExePath, args: ['desktop'], cwd: process.cwd() }
    }

    const goCwd = path.join(__dirname, '..', '..', '..')
    return { command: 'go', args: ['run', './cmd/hermind', 'desktop'], cwd: goCwd }
  }

  function getTempFilePath(pid: number): string {
    return path.join(os.tmpdir(), `hermind-port-${pid}.txt`)
  }

  function cleanupTempFiles(): void {
    try {
      const tmpDir = os.tmpdir()
      const files = fs.readdirSync(tmpDir)
      for (const file of files) {
        if (file.startsWith('hermind-port-')) {
          fs.unlinkSync(path.join(tmpDir, file))
        }
      }
    } catch {
      // ignore cleanup errors
    }
  }

  function tryReadPortFromTempFile(pid: number): number | null {
    try {
      const tempFile = getTempFilePath(pid)
      if (fs.existsSync(tempFile)) {
        const content = fs.readFileSync(tempFile, 'utf-8').trim()
        const parsed = parseInt(content, 10)
        if (!isNaN(parsed) && parsed > 0) {
          return parsed
        }
      }
    } catch {
      // ignore read errors
    }
    return null
  }

  async function pollHealth(targetPort: number): Promise<boolean> {
    for (let i = 0; i < HEALTH_POLL_MAX_ATTEMPTS; i++) {
      if (stopRequested) return false
      try {
        const response = await fetch(`http://127.0.0.1:${targetPort}/health`)
        if (response.status === 200) {
          return true
        }
      } catch {
        // ignore fetch errors
      }
      await new Promise((resolve) => setTimeout(resolve, HEALTH_POLL_INTERVAL_MS))
    }
    return false
  }

  async function handlePortDiscovery(discoveredPort: number, pid: number): Promise<void> {
    if (port !== null) return
    port = discoveredPort

    if (portTimeout) {
      clearTimeout(portTimeout)
      portTimeout = null
    }

    const healthy = await pollHealth(port)
    if (!healthy) {
      killChild()
      if (startReject) {
        startReject(new Error(`Health check failed for port ${port}`))
        startResolve = null
        startReject = null
      }
      return
    }

    setStatus('running')
    retryCount = 0

    if (startResolve) {
      startResolve(port)
      startResolve = null
      startReject = null
    }
  }

  function killChild(): void {
    if (!child) return

    if (sigkillTimeout) {
      clearTimeout(sigkillTimeout)
      sigkillTimeout = null
    }

    child.kill('SIGTERM')

    sigkillTimeout = setTimeout(() => {
      if (child && !child.killed) {
        child.kill('SIGKILL')
      }
    }, SIGKILL_DELAY_MS)
  }

  function spawnProcess(): void {
    if (stopRequested) return

    const { command, args, cwd } = getBinaryInfo()

    setStatus(retryCount > 0 ? 'restarting' : 'starting')

    const proc = spawn(command, args, {
      cwd,
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true
    })

    child = proc
    let stdoutBuffer = ''

    proc.stdout?.on('data', (data: Buffer) => {
      const chunk = data.toString()
      stdoutBuffer += chunk

      const match = READY_PATTERN.exec(stdoutBuffer)
      if (match) {
        const discoveredPort = parseInt(match[1], 10)
        if (!isNaN(discoveredPort)) {
          handlePortDiscovery(discoveredPort, proc.pid ?? 0)
        }
      }
    })

    proc.stderr?.on('data', (data: Buffer) => {
      console.error(`[Go stderr] ${data.toString().trim()}`)
    })

    portTimeout = setTimeout(() => {
      if (port === null && proc.pid) {
        const tempPort = tryReadPortFromTempFile(proc.pid)
        if (tempPort !== null) {
          handlePortDiscovery(tempPort, proc.pid ?? 0)
        }
      }
    }, PORT_DISCOVERY_TIMEOUT_MS)

    proc.on('error', (err) => {
      setStatus('error')
      if (portTimeout) {
        clearTimeout(portTimeout)
        portTimeout = null
      }
      if (startReject) {
        startReject(err)
        startResolve = null
        startReject = null
      }
    })

    proc.on('exit', () => {
      child = null
      if (portTimeout) {
        clearTimeout(portTimeout)
        portTimeout = null
      }

      if (stopRequested) {
        cleanupTempFiles()
        return
      }

      if (port !== null) {
        port = null
        setStatus('error')
      }

      if (retryCount < MAX_RETRIES) {
        retryCount++
        setTimeout(() => {
          if (!stopRequested) {
            spawnProcess()
          }
        }, RETRY_DELAY_MS)
      } else {
        setStatus('error')
        if (startReject) {
          startReject(new Error('Go process failed to start after maximum retries'))
          startResolve = null
          startReject = null
        }
      }
    })
  }

  return {
    start(): Promise<number> {
      if (port !== null) {
        return Promise.resolve(port)
      }

      if (startResolve) {
        return new Promise((resolve, reject) => {
          const check = setInterval(() => {
            if (port !== null) {
              clearInterval(check)
              resolve(port)
            } else if (status === 'error' && !startResolve) {
              clearInterval(check)
              reject(new Error('Go process failed to start'))
            }
          }, 100)
        })
      }

      stopRequested = false
      return new Promise((resolve, reject) => {
        startResolve = resolve
        startReject = reject
        spawnProcess()
      })
    },

    stop(): void {
      stopRequested = true
      retryCount = MAX_RETRIES
      port = null

      if (portTimeout) {
        clearTimeout(portTimeout)
        portTimeout = null
      }

      killChild()

      if (startReject) {
        startReject(new Error('Go process stopped'))
        startResolve = null
        startReject = null
      }

      cleanupTempFiles()
    },

    getStatus(): GoStatus {
      return status
    },

    getPort(): number | null {
      return port
    },

    onStatusChange(callback: (status: GoStatus) => void): () => void {
      statusCallbacks.add(callback)
      return () => {
        statusCallbacks.delete(callback)
      }
    }
  }
}
