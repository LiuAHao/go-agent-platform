const { contextBridge, shell, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('desktopBridge', {
  // 版本信息
  versions: {
    chrome: process.versions.chrome,
    electron: process.versions.electron,
    node: process.versions.node,
  },

  // 打开外部链接
  openExternal(url) {
    if (typeof url === 'string' && url.length > 0) {
      shell.openExternal(url)
    }
  },

  // 选择文件夹
  selectFolder() {
    return ipcRenderer.invoke('select-folder')
  },

  // 显示通知
  showNotification(title, body) {
    return ipcRenderer.invoke('show-notification', { title, body })
  },

  // 获取应用信息
  getAppInfo() {
    return ipcRenderer.invoke('get-app-info')
  },
})
