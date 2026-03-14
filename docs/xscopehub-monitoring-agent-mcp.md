# XScopeHub Monitoring Analysis Agent + MCP Server

## Overview

XScopeHub now exposes a production-oriented monitoring analysis stack with three runtime layers:

1. `observe-gateway`
   Unified query gateway for metrics, logs, traces, and topology evidence.
   Both `llm-ops-agent` and `mcp-server` query observability data only through this service.

2. `llm-ops-agent`
   Monitoring analysis orchestrator.
   It keeps the existing case/orchestrator APIs and adds `POST /analysis/run` for evidence aggregation and structured incident analysis.
   Codex is used as an optional reasoner. If Codex fails, the service falls back to heuristic diagnosis so evidence collection still succeeds.

3. `mcp-server`
   External MCP surface for Codex, OpenClaw, and other MCP clients.
   It now exposes monitoring-focused tools instead of demo tools and forwards tenant/user context to downstream services.

## Repository Additions

- `third_party/codex`
  Git submodule for `openai/codex`.

- `configs/codex/config.toml.example`
  Example project-scoped Codex config with `mcp_servers.xscopehub`.

- `configs/openclaw/x-observability-agent.json5`
  Example OpenClaw ACP agent registration for `x-observability-agent`.

- `scripts/codex/setup-project-home.sh`
  Generates project-local `CODEX_HOME` and `config.toml`.

- `scripts/codex/run-monitor.sh`
  Runs Codex in non-interactive mode against the XScopeHub MCP server.

- `scripts/openclaw/render-xscope-monitor-config.sh`
  Renders the OpenClaw agent config from `.env` and project paths.

- `scripts/openclaw/register-x-observability-agent.sh`
  Performs a real `agents.list / agents.create / agents.update` cycle against OpenClaw Gateway.

- `scripts/smoke-monitor-stack.sh`
  Runs unit tests for the three services and prepares Codex home.

## Runtime Interfaces

### observe-gateway

- Query endpoint: `POST /api/query`
- Supports template-backed queries through:
  - `service_error_rate`
  - `service_latency_p95`
  - `service_error_logs`
  - `service_error_traces`
  - `service_topology_logs`

### llm-ops-agent

- Existing routes remain:
  - `POST /case/create`
  - `PATCH /case/:id/transition`
- New route:
  - `POST /analysis/run`

Sample request:

```json
{
  "goal": "analyze_incident",
  "tenant": "acme-prod",
  "service": "checkout",
  "incident": "error rate increased during the last hour",
  "window": "1h"
}
```

Supported goals:

- `analyze_incident`
- `summarize_service_health`
- `propose_remediation`
- `get_topology`

### MCP Server

Path:

- `POST /mcp`

Optional auth:

- `Authorization: Bearer $XSCOPE_MCP_SERVER_AUTH_TOKEN`

Tools:

- `query_metrics`
- `query_logs`
- `query_traces`
- `get_topology`
- `analyze_incident`
- `summarize_service_health`
- `propose_remediation`

Resources:

- `monitoring_capabilities`
- `analysis_templates`

## Codex Integration

Codex is embedded as a local runtime, not as a public API surface.

Flow:

1. `scripts/codex/setup-project-home.sh` writes a project-local `config.toml`.
2. The config registers `mcp_servers.xscopehub.url = $XSCOPE_MCP_SERVER_URL`.
3. `scripts/codex/run-monitor.sh` executes `codex exec` with the local `CODEX_HOME`.
4. `llm-ops-agent` can also invoke Codex as a reasoner through the `models.codex` section in `config/XOpsAgent.yaml`.

The reasoner is best effort:

- Success: structured diagnosis source is `codex`
- Failure: falls back to `heuristic-fallback`

## OpenClaw Integration

Use `configs/openclaw/x-observability-agent.json5` as the base agent definition.

The important ACP settings are:

```json5
{
  name: "x-observability-agent",
  runtime: {
    type: "acp",
    acp: {
      agent: "codex",
      backend: "acpx",
      mode: "persistent",
      cwd: "/Users/shenlan/workspaces/cloud-neutral-toolkit/x-scope-hub.svc.plus"
    }
  }
}
```

Use `scripts/openclaw/render-xscope-monitor-config.sh` to render environment-specific config and binding examples.
Use `scripts/openclaw/register-x-observability-agent.sh` when you want to actually create or update the agent in Gateway.

## Configuration

Primary environment variables live in `.env.example`.

The key groups are:

- `XSCOPE_OBSERVE_GATEWAY_URL`
- `XSCOPE_LLM_OPS_AGENT_URL`
- `XSCOPE_MCP_SERVER_URL`
- `XSCOPE_MCP_SERVER_AUTH_TOKEN`
- `OBSERVE_GATEWAY_TENANT_HEADER`
- `OBSERVE_GATEWAY_USER_HEADER`
- `XSCOPE_CODEX_*`
- `OPENCLAW_*`

`config/XOpsAgent.yaml` is now wired to:

- `inputs.observe_gateway`
- `models.codex`

`config/observe-gateway.yaml` defines default backend profiles and query templates.

## Local Build And Test

Run the repository-level smoke check:

```bash
./scripts/smoke-monitor-stack.sh
```

Or run components independently:

```bash
make test-obsgw
make test
make test-mcp
make codex-home
```

Manual startup sequence:

```bash
make run-obsgw
cd llm-ops-agent && ./bin/xopsagent --daemon=false --config=../config/XOpsAgent.yaml
make run-mcp
```

## Validation Scope

Implemented validation currently covers:

- `observe-gateway` template resolution and custom header handling
- `llm-ops-agent` evidence collection and Codex fallback behavior
- `mcp-server` bearer auth and upstream forwarding behavior

The smoke script also prepares project-local Codex configuration for MCP access.
