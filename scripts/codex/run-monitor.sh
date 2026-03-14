#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"$ROOT_DIR/scripts/codex/setup-project-home.sh" >/dev/null

export CODEX_HOME="${XSCOPE_CODEX_HOME:-$ROOT_DIR/.runtime/codex-home}"

PROMPT="${*:-Analyze the latest service health signals using the xscopehub MCP server.}"

exec "${XSCOPE_CODEX_COMMAND:-codex}" exec \
  --sandbox "${XSCOPE_CODEX_SANDBOX:-read-only}" \
  --skip-git-repo-check \
  --model "${XSCOPE_CODEX_MODEL:-gpt-5.4}" \
  "$PROMPT"
