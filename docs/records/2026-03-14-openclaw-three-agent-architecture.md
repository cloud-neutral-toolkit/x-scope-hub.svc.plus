# 三仓库专职 Agent + 专属 MCP + OpenClaw Gateway 主控方案

## 决策摘要

本次方案固定保留三个独立仓库，不新增第四个 orchestrator 仓库。

- `x-ops-agent.svc.plus` 对外身份固定为 `xops-agent`
- `x-cloud-flow.svc.plus` 对外身份固定为 `x-automation-agent`
- `x-scope-hub.svc.plus` 对外身份固定为 `x-observability-agent`

唯一主控仍然是 `openclaw-gateway`。它负责 agent 注册表、统一入口和多 agent 协调；当前仓库只输出 observability 域能力。

## 当前仓库定位

- 仓库：`x-scope-hub.svc.plus`
- 默认 `OPENCLAW_AGENT_ID`：`x-observability-agent`
- 默认角色：observability 专职 agent
- 主责任：logs、metrics、traces、topology、alert insight

当前仓库继续保留：

- `llm-ops-agent` 作为业务分析入口
- `mcp-server` 作为独立 MCP server
- `observe-gateway` 作为观测统一查询入口

但对外 agent 身份统一收敛为 `x-observability-agent`，不再沿用 `xscope-monitor`。

## 三仓库总体分工

- `xops-agent`
  - 仓库：`x-ops-agent.svc.plus`
  - incident / remediation / ops judgment
- `x-automation-agent`
  - 仓库：`x-cloud-flow.svc.plus`
  - IaC / playbook / automation planning
- `x-observability-agent`
  - 仓库：`x-scope-hub.svc.plus`
  - observability evidence / insight / validation

边界约束固定如下：

- observability 侧可以返回 logs、metrics、traces、topology、alert insight
- observability 侧可以参与变更前后验证与取证协商
- observability 侧不直接执行 remediation 或基础设施自动化

## Gateway 主控边界

`openclaw-gateway` 是唯一主控，固定负责：

1. agent 注册表
2. 对外统一入口
3. 多 agent 协商与路由

当前仓库不接管 orchestration，不把 OPS 判断或 automation 执行吸收到 observability 域。

## 统一接入契约

当前仓库与另外两个仓库统一采用：

- `OPENCLAW_GATEWAY_URL`
- `OPENCLAW_GATEWAY_TOKEN`
- `OPENCLAW_GATEWAY_PASSWORD`
- `OPENCLAW_AGENT_ID`
- `OPENCLAW_AGENT_NAME`
- `OPENCLAW_AGENT_WORKSPACE`
- `OPENCLAW_AGENT_MODEL`
- `OPENCLAW_REGISTER_ON_START`
- `AI_GATEWAY_URL`
- `AI_GATEWAY_API_KEY`

统一对外运行面：

- 真实注册命令：`make register-openclaw`
- MCP HTTP 入口：`POST /mcp`
- 业务 HTTP API：`llm-ops-agent` 和 `observe-gateway` 保持各自现有职责
- Codex runtime 注入面：通过 `AI_GATEWAY_URL` / `AI_GATEWAY_API_KEY` 映射到 `OPENAI_BASE_URL` / `OPENAI_API_KEY`
- A2A HTTP 入口：
  - `POST /a2a/v1/negotiate`
  - `POST /a2a/v1/tasks`
  - `GET /a2a/v1/tasks/{task_id}`

## A2A 标准最小协议

当前仓库与另外两仓统一采用：

请求字段：

- `from_agent_id`
- `to_agent_id`
- `request_id`
- `intent`
- `goal`
- `context`
- `artifacts`
- `constraints`

响应字段：

- `status`
- `owner_agent_id`
- `summary`
- `required_inputs`
- `result`

允许的 `status` 固定为：

- `accepted`
- `declined`
- `needs_input`
- `completed`

所有 A2A 请求必须带 `request_id`，用于观测查询、agent 日志和 gateway 审计的链路追踪。

## 当前仓库接口约定

### 业务 API

`llm-ops-agent` 继续提供：

- `POST /case/create`
- `PATCH /case/:id/transition`
- `POST /analysis/run`

并新增：

- `POST /a2a/v1/negotiate`
- `POST /a2a/v1/tasks`
- `GET /a2a/v1/tasks/{task_id}`

### MCP

`mcp-server` 继续作为 observability MCP server，专注：

- logs
- metrics
- traces
- topology
- alert / remediation insight

### 真实注册

新增脚本：

```bash
./scripts/openclaw/register-x-observability-agent.sh
```

它直接调用 gateway RPC 完成 `agents.list / agents.create / agents.update`，不再只输出静态 json5 示例。

### A2A

当前仓库实现的是 observability 侧 A2A 协商器：

- 收到 logs / metrics / traces / topology / alert 相关目标时，本域接受
- 收到 deploy / IaC / playbook 类目标时，返回 `needs_input`，要求 `x-automation-agent` 参与
- 收到 incident command / execute / remediate / root cause 指令时，拒绝吞并执行并 handoff 到 `xops-agent`

## 推荐命令

```bash
make run-obsgw
make run-llm-ops-agent
make run-mcp
make register-openclaw
```

## 验收标准

1. `make register-openclaw` 连续执行两次：第一次 `created`，第二次 `updated`
2. `llm-ops-agent` 提供 `/a2a/v1/*`
3. `mcp-server` 继续只输出 observability 域工具
4. 默认 agent 身份固定为 `x-observability-agent`，不再使用 `xscope-monitor`
5. Gateway 可基于 `request_id` 把 observability 证据链和跨 agent 协商链路串起来
