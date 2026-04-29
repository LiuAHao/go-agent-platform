# 本地优先 Agent Studio MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 交付一个可演示、可本地运行的 Agent Studio MVP：用户可以登录、下载和安装平台资源、创建 Agent、配置模型，并在本地聊天执行。

**Architecture:** MVP 采用“云端资源分发 + 本地运行时 + 桌面端壳层”的三段式结构。当前 Go 后端和 React 控制台继续复用，但要明确拆出本地运行时职责，并把 Web 的角色收敛为官网、登录和资源入口。

**Tech Stack:** Go、React、Vite、PostgreSQL、Electron、PowerShell、REST API

---

## MVP 范围

本次 MVP 只覆盖最小闭环，不追求完整平台能力。

### 必须交付

- 桌面端壳层可启动现有控制台
- 本地运行时可承载 Agent、Skill、MCP、模型和聊天执行
- 平台 Skill / MCP 可以浏览、安装到“我的资源”
- 用户可以本地创建 Agent
- 用户可以本地配置模型
- 用户可以进入 Agent 聊天页并发起对话

### 暂不纳入 MVP

- 移动端
- 云端任务主执行
- 完整多设备同步
- AI 自动生成 Skill 的真实后端
- Skill 在线编辑器
- MCP / 模型复杂连通性诊断
- 企业级权限与计费体系

---

### Task 1: 建立 docs 目录规范与 MVP 文档入口

**Files:**
- Create: `docs/README.md`
- Create: `docs/plans/2026-04-24-mvp-implementation.md`
- Modify: `README.md`

**Step 1: 补齐 docs 说明**

在 `docs/README.md` 说明：

- `docs/` 的职责
- `adr/`、`plans/`、`images/` 的用途
- 当前项目已经转向“本地优先 Agent Studio”

**Step 2: 在根 README 中加入文档导航**

把以下入口放到根文档中：

- `设计方案.md`
- `docs/README.md`
- 本计划文档

**Step 3: 自查链接**

确认三个路径都存在，且 README 中的相对路径可以正常点击。

**Step 4: 提交**

```bash
git add docs/README.md docs/plans/2026-04-24-mvp-implementation.md README.md
git commit -m "docs: add mvp implementation plan"
```

---

### Task 2: 明确本地运行时与云端平台边界

**Files:**
- Modify: `设计方案.md`
- Create: `docs/adr/002-local-first-runtime.md`
- Modify: `README.md`

**Step 1: 写一份本地优先 ADR**

在 `docs/adr/002-local-first-runtime.md` 中明确：

- 为什么不做全云端执行
- 为什么桌面端是主产品
- 为什么 Web 不是主执行端
- 为什么移动端只做伴侣端

**Step 2: 在设计方案中补一张职责清单**

把职责拆成三块：

- 云端平台：账号、市场、分发、同步
- 本地运行时：执行、文件、浏览器、模型、聊天
- 桌面端 UI：资源安装、Agent 创建、聊天、设置

**Step 3: 检查是否还有“工作区 / 全云执行”残留表述**

运行：

```powershell
rg -n "工作区|全云端|云端执行|托管运行" README.md 设计方案.md docs
```

预期：

- 旧概念只在历史说明或 ADR 中被解释，不再作为主定位

**Step 4: 提交**

```bash
git add 设计方案.md README.md docs/adr/002-local-first-runtime.md
git commit -m "docs: define local-first runtime boundary"
```

---

### Task 3: 接入 Electron 桌面壳层

**Files:**
- Create: `desktop/package.json`
- Create: `desktop/main.js`
- Create: `desktop/preload.js`
- Create: `desktop/README.md`
- Modify: `README.md`

**Step 1: 新建桌面端模块结构**

创建最小目录：

```text
desktop/
  package.json
  main.js
  preload.js
  README.md
```

**Step 2: 编写 Electron 最小主进程**

要求：

- 启动一个 BrowserWindow
- 开发环境加载 `http://localhost:5173`
- 生产环境加载前端构建产物

**Step 3: 编写 preload**

先保持最小化：

- 不暴露危险 Node 能力
- 仅为后续本地文件选择、系统调用预留桥接入口

**Step 4: 写模块 README**

在 `desktop/README.md` 写清楚：

- 该目录是桌面壳层
- 当前只承载控制台
- 后续将承担本地系统桥接能力

**Step 5: 验证**

至少确认：

- `npm install`
- `npm run dev`

能启动桌面壳加载前端页面

**Step 6: 提交**

```bash
git add desktop README.md
git commit -m "feat: add electron desktop shell"
```

---

### Task 4: 拆分本地运行时配置

**Files:**
- Modify: `cmd/api/*`
- Modify: `internal/config/*`
- Modify: `README.md`
- Create: `docs/adr/003-cloud-vs-local-api.md`

**Step 1: 增加运行模式配置**

建议新增配置项：

- `APP_MODE=cloud|local`
- `API_ROLE=registry|runtime`

**Step 2: 梳理当前 API 中哪些属于本地运行时**

列出并标记：

- Agent
- Session
- Message
- Task
- 本地模型配置
- 本地 MCP 与 Skill 安装后的资源调用

**Step 3: 梳理哪些属于云端平台**

列出并标记：

- 登录
- 平台 Skill 列表
- 平台 MCP 列表
- 版本与更新元数据
- 下载索引

**Step 4: 文档化**

在 `docs/adr/003-cloud-vs-local-api.md` 中写出：

- 现在不必物理拆库
- 但接口职责必须先拆清

**Step 5: 提交**

```bash
git add internal/config cmd/api README.md docs/adr/003-cloud-vs-local-api.md
git commit -m "docs: define cloud and local api roles"
```

---

### Task 5: 收敛前端为桌面端 MVP 交互

**Files:**
- Modify: `web/console/src/App.tsx`
- Modify: `web/console/src/components/LandingPage.tsx`
- Modify: `web/console/src/components/Sidebar.tsx`
- Modify: `web/console/src/components/AgentDashboard.tsx`
- Modify: `web/console/src/styles.css`
- Modify: `web/console/src/components/README.md`

**Step 1: 调整首页产品表达**

首页必须明确：

- 平台统一提供 Skill / MCP
- 桌面端完成本地构建与执行
- Web 主要是入口与市场

**Step 2: 调整控制台文案**

控制台所有核心页面保持：

- 小白优先
- 默认基础配置
- 高级设置折叠

**Step 3: 保持当前四个核心页面稳定**

确认以下页面可用：

- 新建 Agent
- Skill 管理
- MCP 管理
- 模型配置

**Step 4: 验证**

运行：

```powershell
cd .\web\console
npm run build
```

预期：

- 构建成功

**Step 5: 提交**

```bash
git add web/console
git commit -m "feat: align console for desktop mvp"
```

---

### Task 6: 建立本地 Skill 安装最小闭环

**Files:**
- Modify: `internal/app/skill.go`
- Modify: `internal/platform/postgres/*`
- Modify: `internal/platform/memory/*`
- Modify: `web/console/src/components/AgentDashboard.tsx`
- Test: `tests/...`

**Step 1: 明确个人 Skill 最小字段**

MVP 中个人 Skill 只要求：

- 名称
- Slug
- 描述
- 本地文件包元信息

**Step 2: 保存上传后的 Skill 草稿**

前端上传本地文件夹后：

- 生成 Skill 草稿
- 保存基础元信息
- 保存文件清单或 bundle 元数据

**Step 3: 提供下载草稿能力**

保证用户可以把已创建的个人 Skill 下载下来继续调整。

**Step 4: 写最小测试**

覆盖：

- 个人 Skill 创建
- 文件包元信息存储
- 下载草稿所需数据可读

**Step 5: 验证**

运行相关测试，并确保控制台创建流程不报错。

**Step 6: 提交**

```bash
git add internal/app internal/platform web/console tests
git commit -m "feat: add local skill draft flow"
```

---

### Task 7: 建立本地 MCP 安装最小闭环

**Files:**
- Modify: `internal/app/tool.go`
- Modify: `web/console/src/components/AgentDashboard.tsx`
- Test: `tests/...`

**Step 1: 明确 MCP 基础输入**

MVP 创建 MCP 只要求：

- 名称
- 描述
- 绑定 JSON

**Step 2: 高级设置仅做补充**

把以下保持为可选扩展：

- 审批
- Schema
- 平台范围

**Step 3: 增加最小校验**

至少校验：

- `name` 非空
- 绑定 JSON 可解析

**Step 4: 写测试**

覆盖：

- 成功创建 MCP
- JSON 非法时报错

**Step 5: 提交**

```bash
git add internal/app web/console tests
git commit -m "feat: simplify mcp creation flow"
```

---

### Task 8: 建立本地模型配置最小闭环

**Files:**
- Modify: `internal/app/model.go`
- Modify: `internal/domain/model/*`
- Modify: `web/console/src/components/AgentDashboard.tsx`
- Test: `tests/...`

**Step 1: 收敛模型基础字段**

MVP 模型配置默认只保留：

- 模型名称
- 官方模型名称
- API URL
- API Key

**Step 2: 高级设置下沉**

保留但折叠：

- Provider
- 上下文窗口
- 最大输出 Token
- 默认模型

**Step 3: 写测试**

覆盖：

- 基础字段创建成功
- 缺少 URL 或 Key 时报错

**Step 4: 提交**

```bash
git add internal/app internal/domain web/console tests
git commit -m "feat: simplify model configuration flow"
```

---

### Task 9: 建立 Agent 创建到聊天执行闭环

**Files:**
- Modify: `internal/app/agent.go`
- Modify: `internal/app/task.go`
- Modify: `web/console/src/components/AgentDashboard.tsx`
- Test: `tests/...`

**Step 1: 确认 Agent 创建最小字段**

只要求：

- 名称
- 说明
- 模型
- Skill 绑定
- MCP 绑定

其中说明同时写入：

- `description`
- `system_prompt`

**Step 2: 确认聊天页面可进入**

要求：

- 选中 Agent 后进入聊天页
- 没有消息时显示空状态
- 输入框常驻底部

**Step 3: 确认任务执行链可通**

即使底层仍是 mock provider，也要保证：

- 创建 session
- 发送消息
- 生成 assistant 返回

**Step 4: 写测试**

覆盖：

- Agent 创建成功
- 聊天请求成功
- Message 持久化成功

**Step 5: 提交**

```bash
git add internal/app web/console tests
git commit -m "feat: complete agent chat mvp loop"
```

---

### Task 10: 形成 MVP 验收清单

**Files:**
- Create: `docs/mvp-acceptance-checklist.md`
- Modify: `README.md`

**Step 1: 写验收清单**

按用户路径列出：

1. 登录
2. 浏览平台资源
3. 安装 Skill / MCP
4. 配置模型
5. 创建 Agent
6. 进入聊天
7. 发起对话

**Step 2: 写失败路径**

至少包含：

- 没配模型时不可创建 Agent
- MCP JSON 非法时报错
- Skill 上传为空时报错

**Step 3: 在 README 中挂载清单**

方便后续每次迭代都按清单回归。

**Step 4: 提交**

```bash
git add docs/mvp-acceptance-checklist.md README.md
git commit -m "docs: add mvp acceptance checklist"
```

---

## 风险与控制

### 风险 1：过早做移动端

问题：

- 会分散主产品资源
- 会迫使产品妥协成不适合本地执行的模型

控制：

- MVP 不做移动端

### 风险 2：Web 继续承担主执行角色

问题：

- 产品边界混乱
- 与本地资源访问诉求冲突

控制：

- Web 只承担入口与市场角色

### 风险 3：Electron 壳层引入后，前后端边界继续混乱

问题：

- 云端 API 和本地 API 容易继续耦合

控制：

- 先写 ADR 和职责表
- 代码层可晚拆，接口边界必须先拆

### 风险 4：Skill AI 生成接口过早实现

问题：

- 容易把 MVP 拖进复杂生成链

控制：

- MVP 只保留入口，不强求真实生成后端

---

## 计划完成标准

当以下条件全部成立时，可以认为 MVP 计划基本完成：

- 桌面壳层已接入
- 本地运行时职责明确
- Skill / MCP / 模型 / Agent 四条装配链路可走通
- 用户可以从登录到聊天形成闭环
- 文档、README、ADR 与当前产品定位一致

---

Plan complete and saved to `docs/plans/2026-04-24-mvp-implementation.md`. Two execution options:

**1. Subagent-Driven (this session)** - 我在当前会话里按任务逐步实现、逐步验证  
**2. Parallel Session (separate)** - 另开一个执行会话，按计划批量推进并在关键点回查

你要哪一种？  
