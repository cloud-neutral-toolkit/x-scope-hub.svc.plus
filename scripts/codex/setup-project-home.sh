#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CODEX_HOME_DIR="${XSCOPE_CODEX_HOME:-$ROOT_DIR/.runtime/codex-home}"
CONFIG_PATH="${XSCOPE_CODEX_CONFIG_PATH:-$CODEX_HOME_DIR/config.toml}"
MCP_SERVER_ID="${XSCOPE_CODEX_MCP_SERVER_ID:-xscopehub}"
MCP_URL="${XSCOPE_MCP_SERVER_URL:-http://127.0.0.1:8000/mcp}"
TENANT_HEADER="${OBSERVE_GATEWAY_TENANT_HEADER:-X-Tenant}"
USER_HEADER="${OBSERVE_GATEWAY_USER_HEADER:-X-User}"

mkdir -p "$CODEX_HOME_DIR"

cat > "$CONFIG_PATH" <<EOF
model = "${XSCOPE_CODEX_MODEL:-gpt-5.4}"
sandbox_mode = "${XSCOPE_CODEX_SANDBOX:-read-only}"

[mcp_servers.${MCP_SERVER_ID}]
url = "${MCP_URL}"
bearer_token_env_var = "XSCOPE_MCP_SERVER_AUTH_TOKEN"

[mcp_servers.${MCP_SERVER_ID}.env_http_headers]
"${TENANT_HEADER}" = "XSCOPE_DEFAULT_TENANT"
"${USER_HEADER}" = "XSCOPE_DEFAULT_USER"
EOF

echo "Prepared Codex home at $CODEX_HOME_DIR"
echo "Config: $CONFIG_PATH"
