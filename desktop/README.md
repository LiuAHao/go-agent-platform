# Desktop - Electron 桌面客户端

Go Agent Platform 的桌面客户端壳层，基于 Electron 构建。

## 功能

- **Go Runtime 进程管理**：自动启动/停止 Go 后端
- **文件选择桥接**：选择本地文件夹作为 Skill
- **系统通知**：Agent 执行完成通知
- **托盘常驻**：后台运行、快速唤起
- **自动更新**：后续支持

## 目录结构

```
desktop/
├── main.js           # Electron 主进程
├── preload.js        # 预加载脚本
├── package.json      # 依赖配置
├── start.sh          # 一键启动脚本
└── README.md         # 本文件
```

## 快速开始

### 安装依赖

```bash
npm install
```

### 启动开发模式

```bash
npm run dev
```

### 一键启动 (包含后端和前端)

```bash
./start.sh
```

## 主进程 API

### 窗口管理

- `createMainWindow()`：创建主窗口
- `isDevMode()`：判断是否开发模式

### 进程管理

- `startGoBackend()`：启动 Go 后端
- `stopGoBackend()`：停止 Go 后端

### IPC 通道

- `select-folder`：选择文件夹
- `show-notification`：显示通知
- `get-app-info`：获取应用信息

## 预加载 API

通过 `window.desktopBridge` 暴露给渲染进程：

```javascript
// 版本信息
desktopBridge.versions.chrome
desktopBridge.versions.electron
desktopBridge.versions.node

// 打开外部链接
desktopBridge.openExternal(url)

// 选择文件夹
const folder = await desktopBridge.selectFolder()

// 显示通知
await desktopBridge.showNotification(title, body)

// 获取应用信息
const info = await desktopBridge.getAppInfo()
```

## 构建

### 开发模式

```bash
npm run dev
```

### 生产模式

```bash
npm run build
```

## 配置

### 环境变量

- `ELECTRON_RENDERER_URL`：开发服务器地址 (默认 http://localhost:5173)

### Go 后端

- 开发模式：使用项目根目录的 `go-agent-platform` 二进制
- 生产模式：使用 `resources/go-agent-platform` 二进制

## 依赖

- Electron
- Node.js

## 平台支持

- macOS
- Windows
- Linux
