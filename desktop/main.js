const { app, BrowserWindow, shell } = require('electron')
const path = require('node:path')
const net = require('node:net')

const DEV_SERVER_URL = process.env.ELECTRON_RENDERER_URL || 'http://localhost:5173'
const PROD_INDEX_PATH = path.join(__dirname, '..', 'web', 'console', 'dist', 'index.html')

function isDevMode() {
  return !app.isPackaged
}

function waitForServer(url, timeoutMs = 15000) {
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
          reject(new Error(`开发服务器未在 ${timeoutMs}ms 内就绪：${url}`))
          return
        }
        setTimeout(tryConnect, 300)
      })
    }

    tryConnect()
  })
}

async function createMainWindow() {
  const mainWindow = new BrowserWindow({
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
    return mainWindow
  }

  await mainWindow.loadFile(PROD_INDEX_PATH)
  return mainWindow
}

app.whenReady().then(async () => {
  try {
    await createMainWindow()
  } catch (error) {
    console.error('[desktop] 启动桌面端失败：', error)
    app.quit()
  }

  app.on('activate', async () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      await createMainWindow()
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})
