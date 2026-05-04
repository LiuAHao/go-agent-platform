# Go Agent Platform

Go Agent Platform 是一个本地优先的 Agent Studio，目标是让普通用户也能通过简单配置创建、装配和使用自己的 Agent。

平台采用“云端提供资源、本地完成执行”的产品思路：云端统一分发 Skill、MCP 和 Agent 模板，桌面客户端负责本地 Agent 创建、模型配置、能力装配和对话执行。

## 核心特点

- **本地优先执行**
  Agent 的运行、模型配置、文件访问、MCP 调用和 Skill 管理优先在本地客户端完成，减少对全云端运行环境的依赖。

- **统一资源供给**
  平台提供统一的 Skill / MCP 资源目录，用户可以安装到“我的资源”，再装配给不同 Agent 使用。

- **低门槛 Agent 创建**
  新建 Agent 只需要填写名称、说明、模型和所需能力。复杂参数被收纳到高级设置中，避免一开始暴露过多技术细节。

- **桌面端优先**
  当前已经接入 Electron 基础桌面壳层，可以加载现有 React 控制台，为后续本地文件、系统能力和本地运行时桥接做准备。

- **前后端分离**
  后端使用 Go 实现 API、领域模型和存储层，前端使用 React + Vite 构建控制台界面。

## 当前功能

- 平台首页、登录页、注册页
- 用户登录与鉴权
- Agent 创建与列表管理
- Agent 独立聊天页面
- Skill 管理：平台 Skill、我的 Skill、本地上传文件夹入口
- MCP 管理：平台 MCP、我的 MCP、绑定 JSON 配置
- 模型配置：模型名称、官方模型名、API URL、API Key
- Electron 基础桌面客户端
- `memory` 与 `postgres` 两种存储实现

## 技术栈

- 后端：Go
- 前端：React + Vite + TypeScript
- 桌面端：Electron
- 数据库：Memory / PostgreSQL / SQLite (本地)
- 脚本：PowerShell (Windows) / Bash (Mac/Linux)

## 项目结构

```text
go-agent-platform/
├─ cmd/                    # Go 进程入口
├─ desktop/                # Electron 桌面客户端壳层
├─ docs/                   # 文档、ADR、实施计划
├─ internal/               # Go 应用层、领域层、基础设施
├─ migrations/             # 数据库迁移
├─ scripts/                # 启动、测试、构建脚本
├─ tests/                  # 测试
└─ web/console/            # React + Vite 控制台
```

## 快速启动

### 1. 安装前端依赖

```powershell
cd .\web\console
npm install
```

### 2. 安装桌面端依赖

```powershell
cd .\desktop
$env:npm_config_cache = "..\.npm-cache"
npm install
```

如果 Electron 启动时报 `Electron failed to install correctly`，在 `desktop` 目录执行：

```powershell
$env:npm_config_cache = "..\.npm-cache"
npm rebuild electron
```

### 3. 启动后端

**Windows:**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1
```

**Mac/Linux:**

```bash
./scripts/dev.sh
```

默认 API 地址：

```text
http://localhost:8081
```

默认管理员账号：

```text
admin@example.com
ChangeMe123!
```

### 4. 启动 Web 控制台

```powershell
cd .\web\console
npm run dev
```

默认访问地址：

```text
http://localhost:5173
```

### 5. 启动桌面客户端

确认后端已启动后，在项目根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev-desktop.ps1
```

该脚本会启动 Vite 开发服务并打开 Electron 桌面窗口。

## 构建与测试

### 前端构建

```powershell
cd .\web\console
npm run build
```

### Electron 脚本检查

```powershell
node -c .\desktop\main.js
node -c .\desktop\preload.js
```

### 后端测试

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\test.ps1
```

### PostgreSQL 模式

```powershell
$env:STORAGE_DRIVER = "postgres"
$env:POSTGRES_DSN = "postgres://agent:agent@127.0.0.1:5432/agent_platform?sslmode=disable"
$env:POSTGRES_AUTO_MIGRATE = "true"
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1
```

## 文档

- [设计方案.md](设计方案.md)：产品定位、架构方向和实施路线
- [docs/README.md](docs/README.md)：文档目录说明
- [docs/plans/2026-05-04-local-execution-plan.md](docs/plans/2026-05-04-local-execution-plan.md)：本地执行能力实施计划
- [docs/plans/2026-04-24-mvp-implementation.md](docs/plans/2026-04-24-mvp-implementation.md)：MVP 实施计划
- [desktop/README.md](desktop/README.md)：桌面端模块说明
- [web/console/src/components/README.md](web/console/src/components/README.md)：前端组件说明

## 架构方向

### 数据边界

| 类型 | 云端 | 本地 |
|------|------|------|
| Skill | 市场模板、版本、下载地址 | 安装文件、执行环境 |
| MCP | 配置模板、连接参数说明 | Server 进程、密钥 |
| 模型 | 模型名称、API URL | **API Key** |
| Agent | 配置、聊天记录 (同步) | 执行环境 |
| 聊天 | 双向同步 | 本地优先存储 |

### Skill 分层

- **平台 Skill**：云端市场下载 → 本地安装 → Agent 绑定执行
- **本地 Skill**：用户本地文件夹 → 直接引用 → 不上传云端

### MCP 本地运行

云端只存 MCP 配置模板，MCP Server 在用户本机启动，Agent 通过本地 Client 调用。

### 敏感数据隔离

API Key、MCP 密钥等敏感数据**永远不上传云端**，只存本地 SQLite。

## 当前状态

当前项目处于 MVP 前期阶段，已经完成 Web 控制台和 Electron 桌面壳层的基础框架。下一步重点是：

1. 本地存储与云端同步基础
2. Skill 执行能力 (平台下载 + 本地文件夹)
3. MCP 本地工具调用
4. 模型真实调用闭环
