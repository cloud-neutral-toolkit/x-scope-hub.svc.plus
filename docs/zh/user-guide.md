# 使用手册

该仓库更偏向基础设施编排与可观测体系组合，而不是单一应用二进制。

本页用于记录主要用户或运维角色的日常任务、常见流程，以及现有操作文档入口。

## 与当前代码对齐的说明

- 文档目标仓库: `x-scope-hub.svc.plus`
- 仓库类型: `infra-observability`
- 构建与运行依据: repository structure and scripts only
- 主要实现与运维目录: `deploy/`, `ansible/`, `scripts/`, `config/`, `configs/`
- `package.json` 脚本快照: No package.json scripts were detected.

## 需要继续归并的现有文档

- `llm-ops-agent/Orchestrator-Interaction-Contract-API-Guide.md`
- `llm-ops-agent/overview.md`
- `llm-ops-agent/usage.md`

## 本页下一步应补充的内容

- 先描述当前已落地实现，再补充未来规划，避免只写愿景不写现状。
- 术语需要与仓库根 README、构建清单和实际目录保持一致。
- 将上方列出的历史 runbook、spec、子系统说明逐步链接并归并到本页。
- 优先提供面向流程的示例，并确保截图或终端片段与最新 UI/CLI 行为一致。
