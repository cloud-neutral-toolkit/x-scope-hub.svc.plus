#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[1/4] Running observe-gateway tests"
(cd "$ROOT_DIR/observe-gateway" && go test ./...)

echo "[2/4] Running llm-ops-agent tests"
(cd "$ROOT_DIR/llm-ops-agent" && go test ./...)

echo "[3/4] Running mcp-server tests"
(cd "$ROOT_DIR/mcp-server" && go test ./...)

echo "[4/4] Preparing Codex home"
"$ROOT_DIR/scripts/codex/setup-project-home.sh"

echo "Smoke checks passed."
