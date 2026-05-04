# Local Platform

本地存储和执行能力模块，为桌面客户端提供本地优先的数据存储和 Skill/MCP 执行能力。

## 目录结构

```
internal/platform/local/
├── home.go              # 本地存储目录管理
├── store.go             # SQLite 存储实现
├── migrations.go        # 数据库迁移
├── sync.go              # 同步管理器
├── executor.go          # 执行器管理器
├── skill_executor.go    # 本地 Skill 执行器
├── skill_installer.go   # 平台 Skill 安装器
└── *_test.go            # 测试文件
```

## 本地存储目录结构

```
~/.agent-platform/
├── db/
│   └── local.db              # 本地 SQLite 数据库
├── skills/
│   ├── {skill-id-1}/         # 平台 Skill 安装目录
│   │   ├── manifest.json
│   │   ├── install-meta.json
│   │   └── ...
│   └── {skill-id-2}/
├── mcps/
│   └── configs/              # MCP 配置文件
├── models/
│   └── configs.json          # 模型配置 (含敏感信息)
├── agents/
│   └── {agent-id}/
│       └── runtime.json
├── logs/
│   └── executions/
└── cache/                    # 临时缓存
```

## 核心组件

### Home

管理本地存储目录结构，负责创建和访问各类目录。

```go
home, err := local.NewHome("")  // 使用默认目录 ~/.agent-platform/
dbPath := home.DBPath()         // 获取数据库路径
skillsDir := home.SkillsDir()   // 获取 Skill 目录
```

### Store

SQLite 存储实现，实现了 `app.Store` 接口，提供完整的本地数据持久化。

```go
store, err := local.NewStore(home)
defer store.Close()

// 创建种子数据
store.EnsureSeedData(cfg)

// Agent 操作
store.SaveAgent(agent)
store.FindAgentByID(id)
store.ListAgents(workspaceID)

// Skill 操作
store.SaveSkill(skill)
store.InstallSkill(userID, skillID)
store.ListInstalledSkillIDs(userID)

// Model 操作 (API Key 只存本地)
store.SaveModel(model)
store.FindModelByID(id)
```

### SyncManager

同步管理器，追踪本地数据变更并支持与云端同步。

```go
syncMgr := local.NewSyncManager(db)

// 标记数据为待同步
syncMgr.MarkDirty(sync.EntityAgent, agentID)

// 获取待同步的数据
dirtyEntities, _ := syncMgr.GetDirtyEntities()

// 标记已同步
syncMgr.MarkSynced(sync.EntityAgent, agentID)
```

### SkillInstaller

平台 Skill 安装器，负责从云端下载并安装 Skill 到本地。

```go
installer := local.NewSkillInstaller(home)

// 安装 Skill
result, err := installer.Install(skillID, version, downloadURL, checksum)

// 检查是否已安装
installed := installer.IsInstalled(skillID)

// 列出已安装的 Skill
installedList, _ := installer.ListInstalled()
```

### LocalSkillExecutor

本地 Skill 执行器，从本地文件夹加载并执行 Skill。

```go
executor, err := local.LoadLocalSkill(id, name, folderPath, config)

// 执行 Skill
result, err := executor.Execute(ctx, input)

// 健康检查
err = executor.HealthCheck()
```

## 敏感数据隔离

以下数据**永远不上传云端**，只存储在本地 SQLite：

- 模型 API Key
- MCP 连接密钥/Token
- 本地文件路径
- 本地执行日志明细

## 测试

```bash
# 运行本地存储测试
go test ./internal/platform/local/... -v

# 运行所有测试
go test ./... -v
```

## 依赖

- `modernc.org/sqlite` - 纯 Go SQLite 实现，无需 CGO
- 标准库 `database/sql` - 数据库抽象层
