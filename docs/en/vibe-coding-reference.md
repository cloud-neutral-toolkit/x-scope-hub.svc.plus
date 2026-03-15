# Vibe Coding Reference

This repository documents infrastructure orchestration and observability composition rather than a single application binary.

Use this page to align AI-assisted coding prompts, repo boundaries, safe edit rules, and documentation update expectations.

## Current code-aligned notes

- Documentation target: `x-scope-hub.svc.plus`
- Repo kind: `infra-observability`
- Manifest and build evidence: repository structure and scripts only
- Primary implementation and ops directories: `deploy/`, `ansible/`, `scripts/`, `config/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `MCP_SERVER_DESIGN.md`
- `llm-ops-agent/Orchestrator-Interaction-Contract-API-Guide.md`
- `llm-ops-agent/api.md`
- `llm-ops-agent/dual-engine-design.md`
- `llm-ops-agent/orchestrator-test.md`
- `llm-ops-agent/overview.md`
- `llm-ops-agent/postgres-init.md`
- `llm-ops-agent/start.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Review prompt templates and repo rules whenever the project adds new subsystems, protected areas, or mandatory verification steps.
