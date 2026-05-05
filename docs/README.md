# Docs 说明

`docs/` 用于存放项目的中长期设计文档、ADR 和分阶段实施计划。

## 目录约定

- `adr/`
  - 关键架构决策记录
- `plans/`
  - 面向实现的执行计划
- `images/`
  - 文档配图

## 当前文档重点

当前项目已经从"全云端 Agent 托管平台"方向，调整为：

**桌面端为主、本地优先执行、云端负责 Skill / MCP / 模板分发与同步的 Agent 平台。**

后续计划文档会围绕这个方向展开。

## 文档列表

### 设计文档

- [设计方案.md](../设计方案.md)：产品定位、架构方向和实施路线

### 实施计划

- [2026-04-24-mvp-implementation.md](plans/2026-04-24-mvp-implementation.md)：MVP 实施计划
- [2026-05-04-local-execution-plan.md](plans/2026-05-04-local-execution-plan.md)：本地执行能力实施计划
- [2026-05-05-cloud-backend-design.md](plans/2026-05-05-cloud-backend-design.md)：云端后端设计方案

### 验收清单

- [阶段0测试验收清单.md](阶段0测试验收清单.md)：阶段0测试验收清单

## 模块文档

- [desktop/README.md](../desktop/README.md)：桌面端模块说明
- [internal/platform/local/README.md](../internal/platform/local/README.md)：本地存储模块说明
- [web/console/src/components/README.md](../web/console/src/components/README.md)：前端组件说明
