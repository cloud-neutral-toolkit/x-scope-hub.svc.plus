# Deployment

This repository documents infrastructure orchestration and observability composition rather than a single application binary.

Use this page to standardize deployment prerequisites, supported topologies, operational checks, and rollback notes.

## Current code-aligned notes

- Documentation target: `x-scope-hub.svc.plus`
- Repo kind: `infra-observability`
- Manifest and build evidence: repository structure and scripts only
- Primary implementation and ops directories: `deploy/`, `ansible/`, `scripts/`, `config/`, `configs/`
- Package scripts snapshot: No package.json scripts were detected.

## Existing docs to reconcile

- `deployment.md`

## What this page should cover next

- Describe the current implementation rather than an aspirational future-only design.
- Keep terminology aligned with the repository root README, manifests, and actual directories.
- Link deeper runbooks, specs, or subsystem notes from the legacy docs listed above.
- Verify deployment steps against current scripts, manifests, CI/CD flow, and environment contracts before each release.
