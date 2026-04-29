const { contextBridge, shell } = require('electron')

contextBridge.exposeInMainWorld('desktopBridge', {
  versions: {
    chrome: process.versions.chrome,
    electron: process.versions.electron,
    node: process.versions.node,
  },
  openExternal(url) {
    if (typeof url === 'string' && url.length > 0) {
      shell.openExternal(url)
    }
  },
})
