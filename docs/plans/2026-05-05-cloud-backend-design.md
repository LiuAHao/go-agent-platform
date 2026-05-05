# 云端管理面板设计方案

> 基于 2026-05-05 讨论确定

**目标：** 构建完整的云端管理后台，包括管理面板 UI、后端服务、基础设施。

---

## 一、整体架构

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              用户端                                      │
├─────────────────────────────────────────────────────────────────────────┤
│  桌面客户端 (Electron)     │     Web 管理面板 (React)                     │
│  - 本地 Agent 执行         │     - 资源管理                               │
│  - 聊天交互                │     - 用户管理                               │
│  - 本地配置                │     - 数据统计                               │
└─────────────┬───────────────┴───────────────┬─────────────────────────────┘
              │                               │
              ▼                               ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                           API Gateway (Nginx)                            │
└─────────────────────────────────────────────────────────────────────────┘
              │                               │
              ▼                               ▼
┌─────────────────────────────┐   ┌─────────────────────────────────────────┐
│     客户端 API 服务          │   │           管理 API 服务                   │
│     (Go - Port 8081)        │   │           (Go - Port 8082)               │
├─────────────────────────────┤   ├─────────────────────────────────────────┤
│ - 用户认证                  │   │ - 资源管理 (Skill/MCP/模板)              │
│ - Agent CRUD                │   │ - 用户管理                              │
│ - 会话管理                  │   │ - 数据统计                              │
│ - 消息管理                  │   │ - 系统配置                              │
│ - 本地配置同步              │   │ - 审计日志                              │
└─────────────┬───────────────┘   └───────────────────┬─────────────────────┘
              │                                       │
              ▼                                       ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                            共享服务层                                     │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │  MySQL   │  │  Redis   │  │ RabbitMQ │  │  MinIO   │  │  搜索    │  │
│  │  主存储   │  │  缓存    │  │  消息队列 │  │  对象存储 │  │  (可选)  │  │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 二、基础设施选型

### 2.1 MySQL - 主数据库

**用途：** 存储结构化数据

| 数据库 | 用途 |
|--------|------|
| `agent_platform` | 主数据库 |
| `agent_platform_cloud` | 云端专用数据 |

**核心表：**
- users - 用户表
- workspaces - 工作区表
- agents - Agent 配置表
- sessions - 会话表
- messages - 消息表
- skills - Skill 表
- tools - MCP 工具表
- models - 模型配置表
- chat_records - 聊天记录表（云端备份）

### 2.2 Redis - 缓存

**用途：**
- 会话缓存
- Token 黑名单
- 限流计数
- 热点数据缓存
- 发布订阅（实时通知）

**Key 设计：**
```
session:{token}           -> 用户会话
user:{id}:agents          -> 用户 Agent 列表缓存
rate_limit:{ip}:{api}     -> API 限流
cache:skill:{id}          -> Skill 详情缓存
cache:model:{id}          -> 模型配置缓存
pubsub:notifications      -> 实时通知频道
```

### 2.3 RabbitMQ - 消息队列

**用途：**
- 异步任务处理
- 聊天记录备份
- 通知分发
- 数据同步

**队列设计：**
```
chat.backup               -> 聊天记录备份队列
task.execution            -> Agent 任务执行队列
notification.send         -> 通知发送队列
data.sync                 -> 数据同步队列
```

### 2.4 MinIO - 对象存储

**用途：**
- Skill 文件包存储
- 用户头像
- 导出文件
- 日志文件

**Bucket 设计：**
```
skills                    -> Skill 文件包
avatars                   -> 用户头像
exports                   -> 导出文件
logs                      -> 日志文件
```

---

## 三、数据库设计

### 3.1 核心表结构

```sql
-- 用户表
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    avatar_url VARCHAR(500),
    role ENUM('user', 'admin', 'super_admin') DEFAULT 'user',
    status ENUM('active', 'disabled', 'deleted') DEFAULT 'active',
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_email (email),
    INDEX idx_role (role),
    INDEX idx_status (status)
);

-- 工作区表
CREATE TABLE workspaces (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_by VARCHAR(36) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- 工作区成员表
CREATE TABLE workspace_members (
    id VARCHAR(36) PRIMARY KEY,
    workspace_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    role ENUM('owner', 'admin', 'member', 'viewer') DEFAULT 'member',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_workspace_user (workspace_id, user_id),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- Skill 表
CREATE TABLE skills (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    version VARCHAR(20) NOT NULL,
    scope ENUM('platform', 'personal') DEFAULT 'personal',
    category VARCHAR(50),
    icon_url VARCHAR(500),
    file_url VARCHAR(500),
    file_size BIGINT DEFAULT 0,
    checksum VARCHAR(64),
    schema_json JSON,
    config_json JSON,
    download_count INT DEFAULT 0,
    status ENUM('draft', 'published', 'archived') DEFAULT 'draft',
    created_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_slug (slug),
    INDEX idx_scope (scope),
    INDEX idx_status (status),
    INDEX idx_category (category),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- MCP 工具表
CREATE TABLE tools (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    version VARCHAR(20) NOT NULL,
    scope ENUM('platform', 'personal') DEFAULT 'personal',
    category VARCHAR(50),
    transport ENUM('stdio', 'sse', 'http') DEFAULT 'stdio',
    command VARCHAR(500),
    args JSON,
    env_json JSON,
    schema_json JSON,
    config_json JSON,
    download_count INT DEFAULT 0,
    status ENUM('draft', 'published', 'archived') DEFAULT 'draft',
    created_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_slug (slug),
    INDEX idx_scope (scope),
    INDEX idx_status (status),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- 模型配置表
CREATE TABLE models (
    id VARCHAR(36) PRIMARY KEY,
    workspace_id VARCHAR(36),
    name VARCHAR(100) NOT NULL,
    provider VARCHAR(50),
    api_base_url VARCHAR(500),
    api_key_encrypted VARCHAR(500),
    model_key VARCHAR(100) NOT NULL,
    description TEXT,
    context_window INT DEFAULT 0,
    max_output_tokens INT DEFAULT 0,
    capabilities JSON,
    is_default BOOLEAN DEFAULT FALSE,
    created_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- Agent 表
CREATE TABLE agents (
    id VARCHAR(36) PRIMARY KEY,
    workspace_id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    system_prompt TEXT,
    model VARCHAR(100),
    skill_policy JSON,
    tool_policy JSON,
    runtime_policy JSON,
    published_version_id VARCHAR(36),
    status ENUM('draft', 'published', 'archived') DEFAULT 'draft',
    created_by VARCHAR(36),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_workspace (workspace_id),
    INDEX idx_status (status),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- 会话表
CREATE TABLE sessions (
    id VARCHAR(36) PRIMARY KEY,
    workspace_id VARCHAR(36) NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    created_by VARCHAR(36) NOT NULL,
    title VARCHAR(200),
    status ENUM('active', 'archived') DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_agent (agent_id),
    INDEX idx_user (created_by),
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id),
    FOREIGN KEY (agent_id) REFERENCES agents(id),
    FOREIGN KEY (created_by) REFERENCES users(id)
);

-- 消息表
CREATE TABLE messages (
    id VARCHAR(36) PRIMARY KEY,
    session_id VARCHAR(36) NOT NULL,
    role ENUM('user', 'assistant', 'system', 'tool') NOT NULL,
    content TEXT NOT NULL,
    metadata_json JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_session (session_id),
    INDEX idx_created (created_at),
    FOREIGN KEY (session_id) REFERENCES sessions(id)
);

-- 聊天记录备份表（云端）
CREATE TABLE chat_backups (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    encrypted_data LONGTEXT,
    backup_type ENUM('auto', 'manual') DEFAULT 'auto',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id),
    INDEX idx_session (session_id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- 审计日志表
CREATE TABLE audit_logs (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id VARCHAR(36),
    ip_address VARCHAR(45),
    user_agent VARCHAR(500),
    metadata_json JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user (user_id),
    INDEX idx_action (action),
    INDEX idx_created (created_at)
);

-- 用户设置表
CREATE TABLE user_settings (
    user_id VARCHAR(36) PRIMARY KEY,
    retention_max_age_days INT DEFAULT 30,
    retention_max_messages INT DEFAULT 1000,
    retention_max_size_mb BIGINT DEFAULT 500,
    retention_auto_clean BOOLEAN DEFAULT FALSE,
    backup_enabled BOOLEAN DEFAULT FALSE,
    backup_frequency VARCHAR(20) DEFAULT 'manual',
    backup_encrypt BOOLEAN DEFAULT TRUE,
    backup_max_days INT DEFAULT 90,
    last_backup_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- 系统配置表
CREATE TABLE system_configs (
    config_key VARCHAR(100) PRIMARY KEY,
    config_value TEXT,
    description VARCHAR(500),
    updated_by VARCHAR(36),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 统计数据表
CREATE TABLE statistics (
    id VARCHAR(36) PRIMARY KEY,
    stat_date DATE NOT NULL,
    stat_type VARCHAR(50) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    metric_value BIGINT DEFAULT 0,
    metadata_json JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_date_type_metric (stat_date, stat_type, metric_name)
);
```

---

## 四、API 设计

### 4.1 客户端 API (Port 8081)

```
认证相关:
POST   /api/v1/auth/login          - 登录
POST   /api/v1/auth/register       - 注册
POST   /api/v1/auth/logout         - 登出
GET    /api/v1/me                  - 获取当前用户

Agent 相关:
GET    /api/v1/agents              - 列表
POST   /api/v1/agents              - 创建
GET    /api/v1/agents/{id}         - 详情
PUT    /api/v1/agents/{id}         - 更新
DELETE /api/v1/agents/{id}         - 删除

会话相关:
GET    /api/v1/sessions            - 列表
POST   /api/v1/sessions            - 创建
DELETE /api/v1/sessions/{id}       - 删除
GET    /api/v1/sessions/{id}/messages - 消息列表

执行相关:
POST   /api/v1/execute             - 执行 Agent

配置同步:
GET    /api/v1/sync/pull           - 拉取配置
POST   /api/v1/sync/push           - 推送配置
```

### 4.2 管理 API (Port 8082)

```
用户管理:
GET    /admin/api/v1/users         - 用户列表
GET    /admin/api/v1/users/{id}    - 用户详情
PUT    /admin/api/v1/users/{id}    - 更新用户
DELETE /admin/api/v1/users/{id}    - 删除用户

资源管理:
GET    /admin/api/v1/skills        - Skill 列表
POST   /admin/api/v1/skills        - 创建 Skill
PUT    /admin/api/v1/skills/{id}   - 更新 Skill
DELETE /admin/api/v1/skills/{id}   - 删除 Skill

GET    /admin/api/v1/tools         - MCP 工具列表
POST   /admin/api/v1/tools         - 创建 MCP 工具
PUT    /admin/api/v1/tools/{id}    - 更新
DELETE /admin/api/v1/tools/{id}    - 删除

统计:
GET    /admin/api/v1/stats/overview    - 概览统计
GET    /admin/api/v1/stats/users       - 用户统计
GET    /admin/api/v1/stats/skills      - Skill 统计
GET    /admin/api/v1/stats/usage       - 使用统计

系统:
GET    /admin/api/v1/system/config     - 系统配置
PUT    /admin/api/v1/system/config     - 更新配置
GET    /admin/api/v1/system/logs       - 系统日志
```

---

## 五、管理面板页面

### 5.1 页面结构

```
管理面板
├── 仪表盘 (Dashboard)
│   ├── 用户统计
│   ├── 资源统计
│   ├── 使用趋势
│   └── 系统状态
│
├── 用户管理
│   ├── 用户列表
│   ├── 用户详情
│   ├── 角色管理
│   └── 封禁管理
│
├── 资源管理
│   ├── Skill 管理
│   │   ├── 列表
│   │   ├── 创建/编辑
│   │   ├── 版本管理
│   │   └── 审核
│   ├── MCP 工具管理
│   │   ├── 列表
│   │   ├── 创建/编辑
│   │   └── 版本管理
│   └── Agent 模板
│       ├── 列表
│       └── 创建/编辑
│
├── 数据统计
│   ├── 使用统计
│   ├── 下载统计
│   └── 错误统计
│
├── 系统管理
│   ├── 系统配置
│   ├── 审计日志
│   └── 系统监控
│
└── 个人设置
    ├── 账号信息
    └── 密码修改
```

### 5.2 技术栈

- **前端框架：** React + TypeScript
- **UI 组件库：** Ant Design
- **状态管理：** Zustand
- **图表：** ECharts / Recharts
- **路由：** React Router
- **HTTP 客户端：** Axios

---

## 六、目录结构

```
go-agent-platform/
├── cmd/
│   ├── api/              # 客户端 API 服务
│   ├── admin/            # 管理 API 服务
│   └── worker/           # 后台任务处理
│
├── internal/
│   ├── app/              # 应用层
│   ├── domain/           # 领域层
│   ├── platform/         # 基础设施层
│   │   ├── mysql/        # MySQL 存储
│   │   ├── redis/        # Redis 缓存
│   │   ├── rabbitmq/     # RabbitMQ 消息队列
│   │   ├── minio/        # MinIO 对象存储
│   │   └── ...
│   └── transport/        # 传输层
│       ├── http/         # HTTP 处理
│       └── admin/        # 管理 API 处理
│
├── web/
│   ├── console/          # 用户控制台
│   └── admin/            # 管理面板
│
├── deployments/
│   ├── docker-compose.yml
│   ├── mysql/
│   ├── redis/
│   ├── rabbitmq/
│   └── minio/
│
└── migrations/
    ├── 001_init.sql
    ├── 002_*.sql
    └── ...
```

---

## 七、部署方案

### 7.1 Docker Compose

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: agent_platform
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  rabbitmq:
    image: rabbitmq:3-management
    ports:
      - "5672:5672"
      - "15672:15672"
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    volumes:
      - minio_data:/data

  api:
    build:
      context: .
      dockerfile: Dockerfile.api
    ports:
      - "8081:8081"
    depends_on:
      - mysql
      - redis
      - rabbitmq
      - minio

  admin:
    build:
      context: .
      dockerfile: Dockerfile.admin
    ports:
      - "8082:8082"
    depends_on:
      - mysql
      - redis

  worker:
    build:
      context: .
      dockerfile: Dockerfile.worker
    depends_on:
      - mysql
      - redis
      - rabbitmq

volumes:
  mysql_data:
  redis_data:
  rabbitmq_data:
  minio_data:
```

---

## 八、实施计划

### Phase 7.1: 基础设施搭建 (1周)

| 任务 | 描述 |
|------|------|
| 7.1.1 | Docker Compose 配置 |
| 7.1.2 | MySQL 数据库初始化 |
| 7.1.3 | Redis 连接 |
| 7.1.4 | RabbitMQ 连接 |
| 7.1.5 | MinIO 连接 |

### Phase 7.2: 后端 API 实现 (2周)

| 任务 | 描述 |
|------|------|
| 7.2.1 | MySQL 存储实现 |
| 7.2.2 | Redis 缓存层 |
| 7.2.3 | 用户管理 API |
| 7.2.4 | 资源管理 API |
| 7.2.5 | 统计 API |
| 7.2.6 | 系统配置 API |

### Phase 7.3: 管理面板前端 (2周)

| 任务 | 描述 |
|------|------|
| 7.3.1 | 项目初始化 |
| 7.3.2 | 仪表盘页面 |
| 7.3.3 | 用户管理页面 |
| 7.3.4 | 资源管理页面 |
| 7.3.5 | 数据统计页面 |
| 7.3.6 | 系统管理页面 |

### Phase 7.4: 集成测试 (1周)

| 任务 | 描述 |
|------|------|
| 7.4.1 | API 测试 |
| 7.4.2 | 功能测试 |
| 7.4.3 | 性能测试 |
| 7.4.4 | 部署测试 |

---

## 九、安全考虑

1. **认证：** JWT + Redis Token 黑名单
2. **授权：** RBAC 角色权限控制
3. **数据加密：** API Key 等敏感信息加密存储
4. **传输安全：** HTTPS
5. **输入验证：** 参数校验、SQL 注入防护
6. **限流：** API 限流防止滥用
7. **审计：** 关键操作记录审计日志

---

## 十、扩展性考虑

1. **水平扩展：** API 服务无状态，可水平扩展
2. **数据库分库：** 按业务拆分数据库
3. **缓存策略：** 多级缓存（本地 + Redis）
4. **消息队列：** 异步处理耗时操作
5. **对象存储：** 大文件存储
6. **CDN：** 静态资源加速

---

*文档生成时间: 2026-05-05*
