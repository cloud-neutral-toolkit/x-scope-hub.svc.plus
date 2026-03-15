# Design

This repository documents infrastructure orchestration and observability composition rather than a single application binary.

Use this page to consolidate design decisions, ADR-style tradeoffs, and roadmap-sensitive implementation notes.

## Current code-aligned notes

- Documentation target: `x-scope-hub.svc.plus`
- Repo kind: `infra-observability`
- Manifest and build evidence: repository structure and scripts only
- Primary implementation and ops directories: `deploy/`, `ansible/`, `scripts/`, `config/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `MCP_SERVER_DESIGN.md`
- `llm-ops-agent/dual-engine-design.md`
- `observe-bridge/Observability-ETL-Suite-Design-EN.md`
- `observe-bridge/Observability-ETL-Suite-Design-ZH.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Promote one-off implementation notes into reusable design records when behavior, APIs, or deployment contracts change.
