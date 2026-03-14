#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AGENT_ID="${OPENCLAW_AGENT_ID:-x-observability-agent}"
AGENT_NAME="${OPENCLAW_AGENT_NAME:-XScopeHub Observability}"
ACP_AGENT="${OPENCLAW_CODEX_AGENT:-codex}"
ACP_BACKEND="${OPENCLAW_ACP_BACKEND:-acpx}"
CHANNEL="${OPENCLAW_BINDINGS_CHANNEL:-discord}"
ACCOUNT="${OPENCLAW_BINDINGS_ACCOUNT:-default}"
PEER_ID="${OPENCLAW_BINDINGS_PEER_ID:-REPLACE_WITH_PEER_ID}"

cat <<EOF
{
  agents: {
    list: [
      {
        id: "${AGENT_ID}",
        name: "${AGENT_NAME}",
        workspace: "${ROOT_DIR}",
        runtime: {
          type: "acp",
          acp: {
            agent: "${ACP_AGENT}",
            backend: "${ACP_BACKEND}",
            mode: "persistent",
            cwd: "${ROOT_DIR}",
          },
        },
      },
    ],
  },
  bindings: [
    {
      type: "acp",
      agentId: "${AGENT_ID}",
      match: {
        channel: "${CHANNEL}",
        accountId: "${ACCOUNT}",
        peer: { kind: "channel", id: "${PEER_ID}" },
      },
      acp: { label: "${AGENT_ID}-main" },
    },
  ],
}
EOF
