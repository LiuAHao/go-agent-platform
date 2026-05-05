# Go Agent Platform

Go Agent Platform 是一个本地优先的 Agent Studio，目标是让普通用户也能通过简单配置创建、装配和使用自己的 Agent。

平台采用"云端提供资源、本地完成执行"的产品思路：云端统一分发 Skill、MCP 和 Agent 模板，桌面客户端负责本地 Agent 创建、模型配置、能力装配和对话执行。

## 核心特点

- **本地优先执行**
  Agent 的运行、模型配置、文件访问、MCP 调用和 Skill 管理优先在本地客户端完成，减少对全云端运行环境的依赖。

- **统一资源供给**
  平台提供统一的 Skill / MCP 资源目录，用户可以安装到"我的资源"，再装配给不同 Agent 使用。

- **低门槛 Agent 创建**
  新建 Agent 只需要填写名称、说明、模型和所需能力。复杂参数被收纳到高级设置中，避免一开始暴露过多技术细节。

- **桌面端优先**
  当前已经接入 Electron 基础桌面壳层，可以加载现有 React 控制台，为后续本地文件、系统能力和本地运行时桥接做准备。

- **前后端分离**
  后端使用 Go 实现 API、领域模型和存储层，前端使用 React + Vite 构建控制台界面。

## 当前功能

### 核心功能

- 平台首页、登录页、注册页
- 用户登录与鉴权
- Agent 创建与列表管理
- Agent 独立聊天页面
- Skill 管理：平台 Skill、我的 Skill、本地上传文件夹入口
- MCP 管理：平台 MCP、我的 MCP、绑定 JSON 配置
- 模型配置：模型名称、官方模型名、API URL、API Key

### 本地存储与同步

- SQLite 本地存储
- 云端配置同步
- 聊天记录本地存储
- 可选云端备份

### 存储管理

- 存储统计
- 自动清理策略
- 删除会话/消息

### MCP 工具能力

- MCP Client (stdio 传输)
- MCP Server 进程管理
- 工具调用支持

### 模型调用

- OpenAI 兼容 API
- DeepSeek 集成
- Function Calling 支持

### Agent 框架

- Plan-and-Execute + ReAct 融合
- 工具预筛选
- 动态重新规划

### 桌面端

- Electron 桌面客户端
- Go Runtime 进程管理
- 文件选择桥接
- 系统通知
- 托盘常驻

### 云端管理面板

- 管理面板 UI (React + Ant Design)
- 用户管理
- Skill/MCP 资源管理
- 数据统计

## 技术栈

### 后端

- Go 1.25+
- SQLite (本地存储)
- PostgreSQL (可选)
- MySQL (云端)

### 前端

- React 18
- TypeScript
- Vite
- Ant Design (管理面板)

### 桌面端

- Electron

### 基础设施

- MySQL 8.0
- Redis 7
- RabbitMQ 3
- MinIO

## 项目结构

```text
go-agent-platform/
├── cmd/                          # Go 进程入口
│   ├── api/                      # 客户端 API 服务
│   ├── admin/                    # 管理 API 服务
│   └── worker/                   # 后台任务处理
├── desktop/                      # Electron 桌面客户端
├── deployments/                  # 部署配置
│   └── mysql/                    # MySQL 初始化脚本
├── docs/                         # 文档
│   ├── adr/                      # 架构决策记录
│   └── plans/                    # 实施计划
├── internal/                     # Go 应用代码
│   ├── app/                      # 应用层
│   ├── config/                   # 配置
│   ├── domain/                   # 领域层
│   ├── platform/                 # 基础设施层
│   │   ├── local/                # 本地存储
│   │   ├── memory/               # 内存存储
│   │   ├── mysql/                # MySQL 存储
│   │   ├── postgres/             # PostgreSQL 存储
│   │   ├── mcp/                  # MCP 客户端
│   │   └── llm/                  # LLM 集成
│   └── transport/                # 传输层
├── migrations/                   # 数据库迁移
├── scripts/                      # 脚本
├── tests/                        # 测试
└── web/                          # 前端
    ├── console/                  # 用户控制台
    └── admin/                    # 管理面板
```

## 快速启动

### 方式一：本地开发

#### 1. 启动后端

**Mac/Linux:**

```bash
./scripts/dev.sh
```

**Windows:**

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\dev.ps1
```

#### 2. 启动前端控制台

```bash
cd web/console
npm install
npm run dev
```

#### 3. 启动桌面客户端

```bash
cd desktop
npm install
npm run dev
```

### 方式二：Docker Compose (云端服务)

```bash
# 启动基础设施
docker-compose up -d

# 启动管理 API
go run ./cmd/admin

# 启动管理面板
cd web/admin
npm install
npm run dev
```

## 默认账号

| 服务 | 账号 | 密码 |
|------|------|------|
| 用户控制台 | admin@example.com | ChangeMe123! |
| 管理面板 | admin@example.com | ChangeMe123! |
| MySQL | agent | agent123 |
| Redis | - | redis123 |
| RabbitMQ | guest | guest |
| MinIO | minioadmin | minioadmin |

## 访问地址

| 服务 | 地址 |
|------|------|
| 用户控制台 | http://localhost:5173 |
| 管理面板 | http://localhost:5174 |
| 客户端 API | http://localhost:8081 |
| 管理 API | http://localhost:8082 |
| RabbitMQ 管理 | http://localhost:15672 |
| MinIO 控制台 | http://localhost:9001 |

## 架构方向

### 数据边界

| 类型 | 云端 | 本地 |
|------|------|------|
| Skill | 市场模板、版本、下载地址 | 安装文件、执行环境 |
| MCP | 配置模板、连接参数说明 | Server 进程、密钥 |
| 模型 | 模型名称、 API URL | **API Key** |
| Agent | 配置、聊天记录 (同步) | 执行环境 |
| 聊天 | 可选备份 | 本地优先存储 |

### Skill 分层

- **平台 Skill**：云端市场下载 → 本地安装 → Agent 绑定执行
- **本地 Skill**：用户本地文件夹 → 直接引用 → 不上传云端

### MCP 本地运行

云端只存 MCP 配置模板，MCP Server 在用户本机启动，Agent 通过本地 Client 调用。

### 敏感数据隔离

API Key、MCP 密钥等敏感数据**永远不上传云端**，只存本地 SQLite。

### Agent 框架

采用 Plan-and-Execute + ReAct 融合架构：

1. **Plan 阶段**：粗粒度规划，预筛选工具
2. **Execute 阶段**：ReAct 循环执行每个子任务
3. **动态重新规划**：执行过程中可调整计划

## 文档

- [设计方案.md](设计方案.md)：产品定位、架构方向和实施路线
- [docs/README.md](docs/README.md)：文档目录说明
- [docs/plans/2026-05-04-local-execution-plan.md](docs/plans/2026-05-04-local-execution-plan.md)：本地执行能力实施计划
- [docs/plans/2026-05-05-cloud-backend-design.md](docs/plans/2026-05-05-cloud-backend-design.md)：云端后端设计方案
- [desktop/README.md](desktop/README.md)：桌面端模块说明
- [internal/platform/local/README.md](internal/platform/local/README.md)：本地存储模块说明

## 实施进度

| 阶段 | 状态 | 说明 |
|------|------|------|
| Phase 1 | ✅ | 本地存储与同步基础 |
| Phase 2 | ✅ | Skill 执行能力 |
| Phase 3 | ✅ | 存储管理与备份 |
| Phase 4 | ✅ | MCP 工具能力 |
| Phase 5 | ✅ | 模型调用闭环 |
| Phase 5.5 | ✅ | Agent 框架搭建 |
| Phase 6 | ✅ | 桌面端集成 |
| Phase 7 | ✅ | 云端管理面板 |

## 贡献指南

欢迎贡献代码、报告问题或提出建议。

## 许可证

MIT License
