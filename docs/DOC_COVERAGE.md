# Documentation Coverage Matrix

This matrix tracks the bilingual canonical documentation set for `x-scope-hub.svc.plus` and maps it back to the current codebase and older docs.

该矩阵用于跟踪 `x-scope-hub.svc.plus` 的双语规范文档，并将其与当前代码状态和历史文档对应起来。

| Category | EN | ZH | Current status | Existing references | Next check |
| --- | --- | --- | --- | --- | --- |
| Architecture | Yes | Yes | Seeded from current codebase and existing docs. | `architecture.md`<br>`llm-ops-agent/overview.md`<br>`mcp_architecture.md`<br>`records/2026-03-14-openclaw-three-agent-architecture.md`<br>`repository_structure.md`<br>`roadmap.md` | Keep diagrams and ownership notes synchronized with actual directories, services, and integration dependencies. |
| Design | Yes | Yes | Seeded from current codebase and existing docs. | `MCP_SERVER_DESIGN.md`<br>`llm-ops-agent/dual-engine-design.md`<br>`observe-bridge/Observability-ETL-Suite-Design-EN.md`<br>`observe-bridge/Observability-ETL-Suite-Design-ZH.md` | Promote one-off implementation notes into reusable design records when behavior, APIs, or deployment contracts change. |
| Deployment | Yes | Yes | Seeded from current codebase and existing docs. | `deployment.md` | Verify deployment steps against current scripts, manifests, CI/CD flow, and environment contracts before each release. |
| User Guide | Yes | Yes | Seeded from current codebase and existing docs. | `llm-ops-agent/Orchestrator-Interaction-Contract-API-Guide.md`<br>`llm-ops-agent/overview.md`<br>`llm-ops-agent/usage.md` | Prefer workflow-oriented examples and keep screenshots or terminal snippets aligned with the latest UI or CLI behavior. |
| Developer Guide | Yes | Yes | Seeded from current codebase and existing docs. | `llm-ops-agent/Orchestrator-Interaction-Contract-API-Guide.md`<br>`llm-ops-agent/api.md`<br>`llm-ops-agent/orchestrator-test.md`<br>`llm-ops-agent/testing.md`<br>`observe-bridge/api.md`<br>`observe-bridge/observe-bridge-test.md`<br>`observe-bridge/test-cases.md` | Keep setup and test commands tied to actual package scripts, Make targets, or language toolchains in this repository. |
| Vibe Coding Reference | Yes | Yes | Seeded from current codebase and existing docs. | `MCP_SERVER_DESIGN.md`<br>`llm-ops-agent/Orchestrator-Interaction-Contract-API-Guide.md`<br>`llm-ops-agent/api.md`<br>`llm-ops-agent/dual-engine-design.md`<br>`llm-ops-agent/orchestrator-test.md`<br>`llm-ops-agent/overview.md`<br>`llm-ops-agent/postgres-init.md`<br>`llm-ops-agent/start.md` | Review prompt templates and repo rules whenever the project adds new subsystems, protected areas, or mandatory verification steps. |
