# 本地优先 Agent 执行能力实施计划

> 基于 2026-05-04 讨论确定的架构方向
> 更新于 2026-05-04: 细化数据存储策略，增加存储管理功能

**目标：** 让 Agent 在用户本机拥有真实的操控能力，同时保持云端资源分发和配置同步。

**核心原则：**
- 云端 = 资源市场 + 配置同步 + 用户管理 + 可选备份
- 本地 = Agent执行 + Skill运行 + MCP调用 + 模型调用 + 聊天记录
- 同步 = 配置双向同步，聊天记录本地优先、可选备份

---

## 一、数据存储策略

### 1.1 存储分工

```
┌─────────────────────────────────────────────────────────────┐
│                      数据存储策略                            │
├──────────────┬──────────────────┬───────────────────────────┤
│    数据类型   │      云端        │          本地             │
├──────────────┼──────────────────┼───────────────────────────┤
│ 用户账号      │ ✅ 主存储         │ 缓存                      │
│ Agent配置     │ ✅ 主存储         │ 缓存 + 执行配置           │
│ 模型配置      │ ✅ (不含API Key)  │ 完整(含Key)               │
│ Skill/MCP    │ ✅ 市场元数据      │ 安装文件 + 配置           │
│ 聊天记录      │ ⚠️ 可选备份       │ ✅ 主存储                 │
│ 执行结果      │ ⚠️ 可选备份       │ ✅ 主存储                 │
└──────────────┴──────────────────┴───────────────────────────┘

⚠️ = 用户主动开启，默认关闭
```

### 1.2 聊天记录存储策略

**为什么聊天记录存本地：**
1. 聊天记录体积大，同步成本高
2. 聊天内容可能敏感，本地存储更安全
3. 离线时也能查看历史
4. 简化同步架构

**可选云端备份：**
- 为移动端跨设备场景预留
- 用户主动开启，默认关闭
- 备份加密存储
- 新设备可拉取备份恢复

### 1.3 敏感数据本地存储

以下数据**永远不上传云端**：
- 模型 API Key
- MCP 连接密钥/Token
- 本地文件路径
- 本地执行日志明细
- 本地Skill的实际代码

### 1.4 同步范围定义

| 数据类型 | 同步方向 | 同步内容 | 不同步内容 |
|---------|---------|---------|-----------|
| Agent配置 | 双向 | 名称、描述、系统提示词、模型绑定、skill/mcp绑定 | - |
| 模型配置 | 双向 | 模型名称、API URL | **API Key** |
| Skill | 单向(下载) | 元数据、版本号、下载地址 | 执行文件、本地配置 |
| MCP | 单向(下载) | 配置模板、连接参数 | 本地Server进程、密钥 |
| 聊天记录 | 本地+可选备份 | (开启备份时) 消息内容、时间戳 | - |
| 本地Skill | 不同步 | - | 本地文件夹路径、执行文件 |

### 1.5 同步触发机制

```
配置同步：
客户端启动 → 拉取云端最新配置 → 合并本地修改 → 推送本地变更
     ↓
配置变更 → 标记dirty → 延迟2s → 推送云端

聊天备份 (可选)：
聊天结束 → 检查备份开关 → 加密 → 上传云端
```

---

## 二、存储管理功能

### 2.1 存储统计

用户需要了解本地存储占用情况：

```
┌─────────────────────────────────────────────┐
│           存储空间                           │
├─────────────────────────────────────────────┤
│  聊天记录    125 MB    [管理]                │
│  Skill文件    45 MB    [管理]                │
│  缓存文件     12 MB    [清理]                │
│  数据库        8 MB    -                     │
├─────────────────────────────────────────────┤
│  总计        190 MB                          │
└─────────────────────────────────────────────┘
```

### 2.2 删除选项

提供多层次的删除能力：

```
┌─────────────────────────────────────────────┐
│           聊天记录管理                        │
├─────────────────────────────────────────────┤
│  删除单条消息        ← 细粒度                │
│  删除单个会话        ← 常用                  │
│  删除Agent所有会话   ← 清理某个Agent的历史    │
│  清空所有聊天记录    ← 大扫除                │
├─────────────────────────────────────────────┤
│  自动清理策略                                 │
│  ├─ 保留最近 N 天                            │
│  ├─ 保留最近 N 条                            │
│  └─ 超过 N GB 自动提醒                       │
└─────────────────────────────────────────────┘
```

### 2.3 自动清理策略

```go
type RetentionPolicy struct {
    MaxAgeDays   int   `json:"max_age_days"`   // 保留最近N天
    MaxMessages  int   `json:"max_messages"`   // 保留最近N条
    MaxSizeMB    int64 `json:"max_size_mb"`    // 最大存储MB
    AutoClean    bool  `json:"auto_clean"`     // 是否自动清理
}
```

### 2.4 删除前提醒

如果开启了云端备份，删除前提醒用户：

```
┌─────────────────────────────────────────────┐
│  ⚠️ 删除提醒                                 │
├─────────────────────────────────────────────┤
│  此操作将删除 15 个会话 (约 2000 条消息)       │
│                                              │
│  已开启云端备份: 是                           │
│  备份状态: 已备份至 2026-05-03                │
│                                              │
│  删除后本地不可恢复，云端备份仍保留            │
│                                              │
│  [取消]  [确认删除]                           │
└─────────────────────────────────────────────┘
```

---

## 三、备份功能设计

### 3.1 用户设置

```go
type BackupSettings struct {
    Enabled       bool   `json:"enabled"`        // 是否开启云端备份
    Frequency     string `json:"frequency"`      // 实时/每日/手动
    Encrypt       bool   `json:"encrypt"`        // 是否加密备份
    MaxBackupDays int    `json:"max_backup_days"` // 云端保留天数
    LastBackupAt  *time.Time `json:"last_backup_at"`
}
```

### 3.2 备份流程

```
触发备份 (聊天结束/定时/手动)
    ↓
检查备份开关
    ↓ (已开启)
收集待备份消息
    ↓
加密 (如果开启)
    ↓
上传云端
    ↓
更新 last_backup_at
```

### 3.3 恢复流程

```
新设备登录
    ↓
检查云端备份
    ↓ (有备份)
提示用户是否恢复
    ↓ (确认)
下载备份
    ↓
解密 (如果加密)
    ↓
写入本地数据库
```

---

## 四、Skill 架构

### 4.1 Skill 分类

```
┌─────────────────────────────────────────────────────────┐
│                      Skill 分类                          │
├─────────────────────┬───────────────────────────────────┤
│    平台 Skill        │         本地 Skill                │
│  (Platform Skill)    │       (Local Skill)               │
├─────────────────────┼───────────────────────────────────┤
│ 云端市场下载          │ 用户本地文件夹                     │
│ 版本管理              │ 用户自己维护                       │
│ 签名校验              │ 不上传云端                         │
│ 安装到本地目录         │ 直接引用本地路径                   │
└─────────────────────┴───────────────────────────────────┘
```

### 4.2 平台 Skill 生命周期

```
云端市场 → 下载到本地 → 校验签名 → 解压到 ~/.agent-platform/skills/{id}/
    ↓
Agent绑定 → 执行时加载 → 运行在本地
```

### 4.3 本地 Skill 生命周期

```
用户选择文件夹 → 记录路径 → 读取入口文件 → 注册到本地Skill列表
    ↓
Agent绑定 → 执行时直接调用本地路径
```

### 4.4 Skill 执行模型

```go
// Skill 执行接口
type SkillExecutor interface {
    Execute(ctx context.Context, input map[string]any) (map[string]any, error)
    Schema() SkillSchema
    HealthCheck() error
}
```

---

## 五、MCP 架构

### 5.1 MCP 定位

MCP (Model Context Protocol) 是**本地工具协议**，Agent 通过 MCP Server 调用本地能力。

```
┌──────────────────────────────────────────────────────────┐
│                    MCP 架构                              │
├──────────────────────────────────────────────────────────┤
│  云端存储: MCP 配置模板 + 连接参数说明                      │
│  本地运行: MCP Server 进程 (用户自己启动或客户端启动)        │
│  Agent调用: 通过本地 MCP Client 连接本地 MCP Server        │
└──────────────────────────────────────────────────────────┘
```

### 5.2 MCP 执行模型

```go
type MCPClient interface {
    Connect(ctx context.Context) error
    CallTool(ctx context.Context, toolName string, input map[string]any) (map[string]any, error)
    ListTools(ctx context.Context) ([]MCPTool, error)
    Disconnect() error
}
```

---

## 六、本地运行时架构

### 6.1 进程结构

```
Electron App
    ├── Main Process (Node.js)
    │   ├── 窗口管理
    │   ├── 系统桥接 (文件选择、通知等)
    │   └── 本地 Go Runtime 进程管理
    │
    ├── Renderer Process (React)
    │   └── UI 控制台
    │
    └── Go Local Runtime
        ├── Local API Server (:8081)
        ├── Agent Executor
        ├── Skill Manager
        ├── MCP Manager
        ├── Model Connector
        ├── Storage Manager
        └── Local Storage (SQLite)
```

### 6.2 本地 API 职责

```
/api/v1/agents          - Agent CRUD
/api/v1/sessions        - 会话管理
/api/v1/messages        - 消息管理 (本地存储)
/api/v1/models          - 本地模型配置 (Key不上传)
/api/v1/skills          - 本地Skill管理
/api/v1/mcps            - 本地MCP管理
/api/v1/execute         - Agent执行入口
/api/v1/sync            - 云端同步接口
/api/v1/storage         - 存储管理 (统计/清理)
/api/v1/backup          - 备份管理 (开启/恢复/状态)
```

### 6.3 Agent 执行流程

```
用户发送消息
    ↓
创建/获取 Session
    ↓
加载 Agent 配置
    ├── 系统提示词
    ├── 绑定的 Skill 列表
    ├── 绑定的 MCP 列表
    └── 模型配置
    ↓
构建 LLM 请求
    ├── System Prompt
    ├── 历史消息 (从本地加载)
    └── 可用工具列表 (来自 Skill + MCP)
    ↓
调用模型 API
    ↓
处理工具调用 (如果有)
    ├── Skill 执行
    └── MCP 工具调用
    ↓
返回结果
    ↓
保存消息到本地 SQLite
    ↓
检查备份开关 → 如果开启则触发备份
```

---

## 七、实施阶段

### Phase 1: 本地存储与同步基础 (1-2周)

**目标：** 建立本地存储和云端同步的基础设施

| 任务 | 描述 | 产出 |
|------|------|------|
| 1.1 | 定义本地存储目录结构 | `~/.agent-platform/` 目录规范 |
| 1.2 | 实现本地 SQLite 存储 | 完整的本地持久化 |
| 1.3 | 定义同步数据模型 | Agent配置的同步协议 |
| 1.4 | 实现基础同步逻辑 | 配置双向同步 |
| 1.5 | 敏感数据隔离 | API Key 等只存本地 |

**验收标准：**
- 重启客户端数据不丢失
- 登录后能拉取云端 Agent 配置
- API Key 不出现在云端

**状态：✅ 已完成**

---

### Phase 2: Skill 执行能力 (1-2周)

**目标：** Agent 能调用本地 Skill

| 任务 | 描述 | 产出 |
|------|------|------|
| 2.1 | 定义 Skill 执行接口 | `SkillExecutor` 接口 |
| 2.2 | 实现本地 Skill 加载 | 从本地文件夹加载 |
| 2.3 | 实现平台 Skill 安装 | 下载、校验、解压 |
| 2.4 | Skill 与 Agent 绑定 | Agent 配置中引用 Skill |
| 2.5 | Skill 执行调用 | Agent 聊天中触发 Skill |

**验收标准：**
- 可以创建本地 Skill (选择文件夹)
- 可以从市场下载平台 Skill
- Agent 聊天时可以调用绑定的 Skill

**状态：✅ 已完成**

---

### Phase 3: 存储管理与备份 (1周)

**目标：** 用户可以管理本地存储，可选备份聊天记录到云端

| 任务 | 描述 | 产出 |
|------|------|------|
| 3.1 | 存储统计接口 | 获取各类数据占用空间 |
| 3.2 | 删除接口 | 删除消息/会话/Agent历史 |
| 3.3 | 自动清理策略 | 按天数/条数/大小自动清理 |
| 3.4 | 备份设置接口 | 开启/关闭/配置备份 |
| 3.5 | 备份上传逻辑 | 加密并上传聊天记录 |
| 3.6 | 备份恢复逻辑 | 新设备拉取并恢复 |

**验收标准：**
- 可以查看存储占用统计
- 可以删除指定聊天记录
- 可以开启云端备份
- 新设备可以恢复备份

---

### Phase 4: MCP 工具能力 (1-2周)

**目标：** Agent 能通过 MCP 调用本地工具

| 任务 | 描述 | 产出 |
|------|------|------|
| 4.1 | 定义 MCP Client 接口 | `MCPClient` 接口 |
| 4.2 | 实现 stdio 传输 | 启动本地 MCP Server 进程 |
| 4.3 | 实现 MCP 工具调用 | Agent 聊天中触发 MCP 工具 |
| 4.4 | MCP 配置管理 | 保存用户填写的连接参数 |
| 4.5 | MCP 健康检查 | 检测 Server 是否可用 |

**验收标准：**
- 可以配置本地 MCP Server
- Agent 聊天时可以调用 MCP 工具
- MCP Server 进程可以启动/停止

---

### Phase 5: 模型调用闭环 (1周)

**目标：** 真实调用 LLM API

| 任务 | 描述 | 产出 |
|------|------|------|
| 5.1 | 实现 OpenAI 兼容调用 | 支持 OpenAI/兼容 API |
| 5.2 | 工具调用协议 | Function Calling 支持 |
| 5.3 | 流式响应 | SSE 流式输出 |
| 5.4 | 错误处理 | 超时、限流、认证失败 |

**验收标准：**
- 配置 API Key 后可以真实对话
- 支持工具调用 (Skill/MCP)
- 支持流式输出

---

### Phase 6: 桌面端集成 (1周)

**目标：** Electron 与本地 Runtime 深度集成

| 任务 | 描述 | 产出 |
|------|------|------|
| 6.1 | Go Runtime 进程管理 | Electron 启动/停止 Go 进程 |
| 6.2 | 文件选择桥接 | 选择本地 Skill 文件夹 |
| 6.3 | 系统通知 | Agent 执行完成通知 |
| 6.4 | 托盘与快捷键 | 后台运行、快速唤起 |

**验收标准：**
- 启动 Electron 自动启动 Go Runtime
- 可以通过系统对话框选择文件夹
- Agent 完成任务后系统通知

---

### Phase 7: 云端管理面板 (2周)

**目标：** 运营可以管理平台资源

| 任务 | 描述 | 产出 |
|------|------|------|
| 7.1 | 管理面板前端 | React 管理后台 |
| 7.2 | Skill/MCP 上架 | 上传、审核、发布 |
| 7.3 | 版本管理 | 版本号、更新日志 |
| 7.4 | 用户管理 | 列表、禁用、角色 |
| 7.5 | 下载统计 | 安装量、使用量 |

**验收标准：**
- 可以上架新 Skill/MCP
- 可以发布新版本
- 可以查看下载统计

---

## 八、目录结构规划

```
~/.agent-platform/
├── db/
│   └── local.db              # 本地 SQLite
├── skills/
│   ├── {skill-id-1}/         # 平台 Skill 安装目录
│   │   ├── manifest.json
│   │   ├── entry.js
│   │   └── ...
│   └── {skill-id-2}/
├── mcps/
│   └── configs/              # MCP 配置文件
│       ├── filesystem.json
│       └── github.json
├── models/
│   └── configs.json          # 模型配置 (含本地Key)
├── agents/
│   └── {agent-id}/
│       └── runtime.json      # Agent 运行时配置
├── backup/
│   └── {backup-id}.enc       # 加密备份文件 (如果开启)
└── logs/
    └── executions/           # 执行日志
```

---

## 九、关键接口定义

### 9.1 配置同步接口

```go
// 云端 API - 只同步配置，不同步聊天记录
POST /api/v1/sync/pull     // 拉取云端配置
POST /api/v1/sync/push     // 推送本地配置变更
GET  /api/v1/sync/status   // 同步状态

// 请求/响应结构
type SyncPullRequest struct {
    LastSyncAt time.Time `json:"last_sync_at"`
}

type SyncPullResponse struct {
    Agents []Agent `json:"agents"`  // Agent配置
    // 注意：不包含 Messages
}

type SyncPushRequest struct {
    Agents []Agent `json:"agents"`  // Agent配置
    // 注意：不包含 Messages
}
```

### 9.2 备份接口

```go
// 本地 API
GET  /api/v1/backup/settings      // 获取备份设置
PUT  /api/v1/backup/settings      // 更新备份设置
POST /api/v1/backup/trigger       // 手动触发备份
POST /api/v1/backup/restore       // 恢复备份
GET  /api/v1/backup/status        // 备份状态

// 备份设置
type BackupSettings struct {
    Enabled       bool       `json:"enabled"`
    Frequency     string     `json:"frequency"`      // realtime/daily/manual
    Encrypt       bool       `json:"encrypt"`
    MaxBackupDays int        `json:"max_backup_days"`
    LastBackupAt  *time.Time `json:"last_backup_at"`
}
```

### 9.3 存储管理接口

```go
// 本地 API
GET  /api/v1/storage/stats              // 获取存储统计
POST /api/v1/storage/clean              // 执行清理
GET  /api/v1/storage/retention          // 获取保留策略
PUT  /api/v1/storage/retention          // 更新保留策略

// 存储统计
type StorageStats struct {
    ChatMessages  int64 `json:"chat_messages"`   // 聊天记录数
    ChatSizeMB    int64 `json:"chat_size_mb"`    // 聊天占用MB
    SkillsCount   int   `json:"skills_count"`    // Skill数量
    SkillsSizeMB  int64 `json:"skills_size_mb"`  // Skill占用MB
    CacheSizeMB   int64 `json:"cache_size_mb"`   // 缓存占用MB
    DatabaseSizeMB int64 `json:"database_size_mb"` // 数据库占用MB
    TotalSizeMB   int64 `json:"total_size_mb"`   // 总占用MB
}

// 保留策略
type RetentionPolicy struct {
    MaxAgeDays  int   `json:"max_age_days"`
    MaxMessages int   `json:"max_messages"`
    MaxSizeMB   int64 `json:"max_size_mb"`
    AutoClean   bool  `json:"auto_clean"`
}

// 清理结果
type CleanupResult struct {
    DeletedSessions int   `json:"deleted_sessions"`
    DeletedMessages int   `json:"deleted_messages"`
    FreedBytes      int64 `json:"freed_bytes"`
}
```

### 9.4 消息删除接口

```go
// 本地 API
DELETE /api/v1/messages/{id}                    // 删除单条消息
DELETE /api/v1/sessions/{id}                    // 删除单个会话
DELETE /api/v1/agents/{id}/sessions             // 删除Agent所有会话
DELETE /api/v1/messages                         // 清空所有 (需要确认)
```

### 9.5 Skill 市场接口

```go
// 云端 API
GET  /api/v1/marketplace/skills           // 列表
GET  /api/v1/marketplace/skills/{id}      // 详情
GET  /api/v1/marketplace/skills/{id}/download // 下载

// 本地 API
GET    /api/v1/skills                // 已安装列表
POST   /api/v1/skills                // 创建本地 Skill
DELETE /api/v1/skills/{id}           // 删除
POST   /api/v1/skills/{id}/install   // 从市场安装
```

### 9.6 Agent 执行接口

```go
// 本地 API
POST /api/v1/execute

type ExecuteRequest struct {
    AgentID string `json:"agent_id"`
    Message string `json:"message"`
}

// SSE 流响应
type ExecuteEvent struct {
    Type    string `json:"type"`    // "text", "tool_call", "tool_result", "error"
    Content string `json:"content"`
}
```

---

## 十、风险与应对

| 风险 | 影响 | 应对 |
|------|------|------|
| 本地存储空间不足 | 用户体验差 | 存储统计、自动清理、空间不足提醒 |
| 聊天记录丢失 | 用户数据丢失 | 可选云端备份、本地定期备份提示 |
| 备份安全 | 隐私泄露 | 加密备份、端到端加密 |
| 本地 Skill 安全性 | 恶意代码执行 | 沙箱隔离、权限声明、用户确认 |
| MCP Server 稳定性 | 进程崩溃影响 Agent | 进程隔离、超时机制、自动重启 |
| 同步冲突 | 数据不一致 | 时间戳优先、冲突提示、手动解决 |
| 离线可用性 | 无网时功能受限 | 本地优先、队列同步、降级策略 |
| 跨平台兼容 | Windows/Mac/Linux 差异 | 抽象层、条件编译、CI 多平台 |

---

## 十一、移动端适配预留

为未来移动端客户端预留的能力：

1. **备份恢复** - 移动端登录后可拉取桌面端备份的聊天记录
2. **配置同步** - Agent配置、模型配置自动同步
3. **轻量存储** - 移动端可设置更短的保留策略
4. **通知推送** - Agent任务完成通知

---

## 十二、下一步行动

1. **确认本计划** - 与你对齐优先级和时间线
2. **开始 Phase 3** - 存储管理与备份功能
3. **迭代推进** - 每个 Phase 完成后验收

---

*计划更新时间: 2026-05-04*
*更新内容: 细化数据存储策略，增加存储管理与备份功能*
