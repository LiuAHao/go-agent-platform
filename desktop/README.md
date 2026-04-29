# 桌面端模块说明

`desktop/` 是 Go Agent Platform 的桌面客户端壳层，当前基于 Electron 实现。

## 当前职责

当前桌面端模块只承担最小壳层职责：

- 启动桌面窗口
- 开发环境加载本地前端调试地址
- 生产环境加载前端构建产物
- 通过 `preload` 预留后续本地系统桥接入口

## 当前不负责的事情

目前这个模块还**不直接承担**以下逻辑：

- 后端 API
- 本地运行时执行
- 系统文件桥接
- MCP 本地进程拉起
- 自动更新
- 桌面通知

这些能力会在后续迭代中逐步接入。

## 目录说明

- `main.js`
  - Electron 主进程入口
- `preload.js`
  - 前端与桌面能力之间的安全桥接层
- `package.json`
  - 桌面端依赖与启动脚本

## 本地开发

先安装桌面端依赖：

```powershell
cd .\desktop
npm install
```

然后有两种启动方式：

### 方式 1：手动启动

先启动前端：

```powershell
cd .\web\console
npm run dev
```

再启动桌面壳：

```powershell
cd .\desktop
npm run dev
```

### 方式 2：使用根目录脚本

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev-desktop.ps1
```

## 生产模式

先构建前端：

```powershell
cd .\web\console
npm run build
```

再启动桌面壳：

```powershell
cd .\desktop
npm run start
```

## 后续演进方向

后续建议按以下顺序扩展：

1. 文件选择和本地目录桥接
2. 本地运行时 API 桥接
3. 本地 MCP / Skill 安装目录管理
4. 自动更新
5. 系统托盘与通知
