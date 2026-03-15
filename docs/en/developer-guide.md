# Developer Guide

This repository documents infrastructure orchestration and observability composition rather than a single application binary.

Use this page to document local setup, project structure, test surfaces, and contribution conventions tied to the current codebase.

## Current code-aligned notes

- Documentation target: `x-scope-hub.svc.plus`
- Repo kind: `infra-observability`
- Manifest and build evidence: repository structure and scripts only
- Primary implementation and ops directories: `deploy/`, `ansible/`, `scripts/`, `config/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `llm-ops-agent/Orchestrator-Interaction-Contract-API-Guide.md`
- `llm-ops-agent/api.md`
- `llm-ops-agent/orchestrator-test.md`
- `llm-ops-agent/testing.md`
- `observe-bridge/api.md`
- `observe-bridge/observe-bridge-test.md`
- `observe-bridge/test-cases.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Keep setup and test commands tied to actual package scripts, Make targets, or language toolchains in this repository.
