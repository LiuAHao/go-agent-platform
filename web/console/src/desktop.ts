// 桌面端 API 封装
// 在 Electron 环境中使用 desktopBridge，否则返回默认值

interface DesktopBridge {
  versions: {
    chrome: string
    electron: string
    node: string
  }
  openExternal: (url: string) => void
  selectFolder: () => Promise<string | null>
  showNotification: (title: string, body: string) => Promise<boolean>
  getAppInfo: () => Promise<{
    version: string
    name: string
    platform: string
    arch: string
    goBackendRunning: boolean
  }>
}

// 获取桌面端 Bridge
function getDesktopBridge(): DesktopBridge | null {
  if (typeof window !== 'undefined' && 'desktopBridge' in window) {
    return (window as any).desktopBridge
  }
  return null
}

// 是否在 Electron 环境中
export function isElectron(): boolean {
  return getDesktopBridge() !== null
}

// 选择文件夹
export async function selectFolder(): Promise<string | null> {
  const bridge = getDesktopBridge()
  if (!bridge) {
    console.warn('不在 Electron 环境中，无法选择文件夹')
    return null
  }
  return bridge.selectFolder()
}

// 显示通知
export async function showNotification(title: string, body: string): Promise<boolean> {
  const bridge = getDesktopBridge()
  if (!bridge) {
    console.warn('不在 Electron 环境中，无法显示通知')
    return false
  }
  return bridge.showNotification(title, body)
}

// 获取应用信息
export async function getAppInfo() {
  const bridge = getDesktopBridge()
  if (!bridge) {
    return {
      version: 'web',
      name: 'Go Agent Platform',
      platform: 'web',
      arch: 'web',
      goBackendRunning: true,
    }
  }
  return bridge.getAppInfo()
}

// 打开外部链接
export function openExternal(url: string) {
  const bridge = getDesktopBridge()
  if (bridge) {
    bridge.openExternal(url)
  } else {
    window.open(url, '_blank')
  }
}

// 获取版本信息
export function getVersions() {
  const bridge = getDesktopBridge()
  if (!bridge) {
    return {
      chrome: 'N/A',
      electron: 'N/A',
      node: 'N/A',
    }
  }
  return bridge.versions
}
