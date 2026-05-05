const { app, BrowserWindow, shell, ipcMain, dialog, Notification, Tray, Menu, nativeImage } = require('electron')
const path = require('node:path')
const net = require('node:net')
const { spawn } = require('node:child_process')

const DEV_SERVER_URL = process.env.ELECTRON_RENDERER_URL || 'http://localhost:5173'
const PROD_INDEX_PATH = path.join(__dirname, '..', 'web', 'console', 'dist', 'index.html')
const GO_API_PORT = 8081

let mainWindow = null
let tray = null
let goProcess = null

function isDevMode() {
  return !app.isPackaged
}

function getGoBinaryPath() {
  if (isDevMode()) {
    return path.join(__dirname, '..', 'go-agent-platform')
  }
  return path.join(process.resourcesPath, 'go-agent-platform')
}

// 等待服务器就绪
function waitForServer(url, timeoutMs = 30000) {
  const { hostname, port } = new URL(url)
  const startedAt = Date.now()

  return new Promise((resolve, reject) => {
    function tryConnect() {
      const socket = net.createConnection({ host: hostname, port: Number(port) }, () => {
        socket.end()
        resolve()
      })

      socket.on('error', () => {
        socket.destroy()
        if (Date.now() - startedAt > timeoutMs) {
          reject(new Error(`服务器未在 ${timeoutMs}ms 内就绪：${url}`))
          return
        }
        setTimeout(tryConnect, 300)
      })
    }

    tryConnect()
  })
}

// 启动 Go 后端
async function startGoBackend() {
  if (goProcess) {
    console.log('[desktop] Go 后端已在运行')
    return
  }

  const goBinary = getGoBinaryPath()
  console.log('[desktop] 启动 Go 后端:', goBinary)

  goProcess = spawn(goBinary, [], {
    cwd: path.dirname(goBinary),
    env: { ...process.env, HTTP_ADDR: `:${GO_API_PORT}` },
    stdio: ['pipe', 'pipe', 'pipe'],
  })

  goProcess.stdout.on('data', (data) => {
    console.log('[go]', data.toString())
  })

  goProcess.stderr.on('data', (data) => {
    console.error('[go]', data.toString())
  })

  goProcess.on('close', (code) => {
    console.log('[desktop] Go 后端已退出，代码:', code)
    goProcess = null
  })

  goProcess.on('error', (err) => {
    console.error('[desktop] 启动 Go 后端失败:', err)
    goProcess = null
  })

  // 等待后端就绪
  await waitForServer(`http://localhost:${GO_API_PORT}`)
  console.log('[desktop] Go 后端已就绪')
}

// 停止 Go 后端
function stopGoBackend() {
  if (goProcess) {
    console.log('[desktop] 停止 Go 后端')
    goProcess.kill()
    goProcess = null
  }
}

// 创建主窗口
async function createMainWindow() {
  mainWindow = new BrowserWindow({
    width: 1440,
    height: 920,
    minWidth: 1100,
    minHeight: 760,
    title: 'Go Agent Platform',
    backgroundColor: '#f4f7f3',
    autoHideMenuBar: true,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  })

  mainWindow.webContents.setWindowOpenHandler(({ url }) => {
    shell.openExternal(url)
    return { action: 'deny' }
  })

  if (isDevMode()) {
    await waitForServer(DEV_SERVER_URL)
    await mainWindow.loadURL(DEV_SERVER_URL)
  } else {
    await mainWindow.loadFile(PROD_INDEX_PATH)
  }

  mainWindow.on('closed', () => {
    mainWindow = null
  })

  return mainWindow
}

// 创建托盘
function createTray() {
  const icon = nativeImage.createFromNamedImage('NSImageNameApplicationIcon', [])
  tray = new Tray(icon.isEmpty() ? path.join(__dirname, 'icon.png') : icon)

  const contextMenu = Menu.buildFromTemplate([
    {
      label: '显示主窗口',
      click: () => {
        if (mainWindow) {
          mainWindow.show()
          mainWindow.focus()
        }
      },
    },
    { type: 'separator' },
    {
      label: '退出',
      click: () => {
        stopGoBackend()
        app.quit()
      },
    },
  ])

  tray.setToolTip('Go Agent Platform')
  tray.setContextMenu(contextMenu)

  tray.on('click', () => {
    if (mainWindow) {
      mainWindow.show()
      mainWindow.focus()
    }
  })
}

// IPC 处理：选择文件夹
ipcMain.handle('select-folder', async () => {
  const result = await dialog.showOpenDialog(mainWindow, {
    properties: ['openDirectory'],
  })

  if (result.canceled) {
    return null
  }

  return result.filePaths[0]
})

// IPC 处理：发送通知
ipcMain.handle('show-notification', async (event, { title, body }) => {
  const notification = new Notification({
    title: title || 'Go Agent Platform',
    body: body,
  })

  notification.show()

  notification.on('click', () => {
    if (mainWindow) {
      mainWindow.show()
      mainWindow.focus()
    }
  })

  return true
})

// IPC 处理：获取应用信息
ipcMain.handle('get-app-info', () => {
  return {
    version: app.getVersion(),
    name: app.getName(),
    platform: process.platform,
    arch: process.arch,
    goBackendRunning: goProcess !== null,
  }
})

// 应用就绪
app.whenReady().then(async () => {
  try {
    // 启动 Go 后端
    await startGoBackend()

    // 创建主窗口
    await createMainWindow()

    // 创建托盘
    createTray()
  } catch (error) {
    console.error('[desktop] 启动失败:', error)
    app.quit()
  }

  app.on('activate', async () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      await createMainWindow()
    }
  })
})

// 所有窗口关闭
app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    stopGoBackend()
    app.quit()
  }
})

// 应用退出前
app.on('before-quit', () => {
  stopGoBackend()
})
