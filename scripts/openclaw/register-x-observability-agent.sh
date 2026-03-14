#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env}"

read_env_value() {
  python3 - "$ENV_FILE" "$@" <<'PY'
import pathlib
import re
import sys

def normalize(value: str) -> str:
    return re.sub(r"[^a-z0-9]", "", value.strip().lower())

path = pathlib.Path(sys.argv[1])
keys = {normalize(item) for item in sys.argv[2:] if item.strip()}
if not path.is_file():
    raise SystemExit(0)

for raw_line in path.read_text().splitlines():
    line = raw_line.strip()
    if not line or line.startswith("#"):
        continue
    sep = next((idx for idx, ch in enumerate(line) if ch in "=:"), -1)
    if sep <= 0:
        continue
    key = line[:sep].strip().rstrip(",").strip().strip("\"'")
    value = line[sep + 1 :].strip().rstrip(",").strip().strip("\"'")
    if normalize(key) in keys and value:
        print(value)
        break
PY
}

OPENCLAW_COMMAND="${OPENCLAW_COMMAND:-openclaw}"
OPENCLAW_URL="${OPENCLAW_GATEWAY_URL:-$(read_env_value OPENCLAW_GATEWAY_URL remote remote-url gateway-remote-url)}"
OPENCLAW_TOKEN="${OPENCLAW_GATEWAY_TOKEN:-$(read_env_value OPENCLAW_GATEWAY_TOKEN remote-token gateway-auth-token)}"
OPENCLAW_PASSWORD="${OPENCLAW_GATEWAY_PASSWORD:-$(read_env_value OPENCLAW_GATEWAY_PASSWORD remote-password)}"
AGENT_ID="${OPENCLAW_AGENT_ID:-$(read_env_value OPENCLAW_AGENT_ID)}"
AGENT_NAME="${OPENCLAW_AGENT_NAME:-$(read_env_value OPENCLAW_AGENT_NAME)}"
AGENT_WORKSPACE="${OPENCLAW_AGENT_WORKSPACE:-$(read_env_value OPENCLAW_AGENT_WORKSPACE)}"
AGENT_MODEL="${OPENCLAW_AGENT_MODEL:-$(read_env_value OPENCLAW_AGENT_MODEL)}"

AGENT_ID="${AGENT_ID:-x-observability-agent}"
AGENT_NAME="${AGENT_NAME:-XScopeHub Observability}"
AGENT_WORKSPACE="${AGENT_WORKSPACE:-$ROOT_DIR}"

gateway_call() {
  local method="$1"
  local params="$2"
  local args=("$OPENCLAW_COMMAND" "gateway" "call" "$method" "--json" "--params" "$params")
  if [[ -n "$OPENCLAW_URL" ]]; then
    args+=("--url" "$OPENCLAW_URL")
  fi
  if [[ -n "$OPENCLAW_TOKEN" ]]; then
    args+=("--token" "$OPENCLAW_TOKEN")
  fi
  if [[ -n "$OPENCLAW_PASSWORD" ]]; then
    args+=("--password" "$OPENCLAW_PASSWORD")
  fi
  "${args[@]}"
}

NORMALIZED_NAME="$(python3 - "$AGENT_NAME" <<'PY'
import re, sys
value = sys.argv[1].strip().lower()
value = value.replace(" ", "-").replace("/", "-").replace(":", "-").replace(".", "-")
parts = re.split(r"[^a-z0-9_-]+", value)
parts = [p for p in parts if p]
print("-".join(parts))
PY
)"
CREATE_NAME="$AGENT_NAME"
if [[ "$NORMALIZED_NAME" != "${AGENT_ID,,}" ]]; then
  CREATE_NAME="$AGENT_ID"
fi

LIST_JSON="$(gateway_call "agents.list" '{}')"
if python3 - "$AGENT_ID" "$LIST_JSON" <<'PY'
import json, sys
agent_id = sys.argv[1].strip().lower()
payload = json.loads(sys.argv[2])
agents = payload.get("agents", [])
for item in agents:
    if str(item.get("id", "")).strip().lower() == agent_id:
        raise SystemExit(0)
raise SystemExit(1)
PY
then
  UPDATE_PARAMS="$(python3 - "$AGENT_ID" "$AGENT_NAME" "$AGENT_WORKSPACE" "$AGENT_MODEL" <<'PY'
import json, sys
payload = {
  "agentId": sys.argv[1],
  "name": sys.argv[2],
  "workspace": sys.argv[3],
}
if sys.argv[4]:
  payload["model"] = sys.argv[4]
print(json.dumps(payload))
PY
)"
  gateway_call "agents.update" "$UPDATE_PARAMS" >/dev/null
  printf '{"operation":"updated","agent_id":"%s","workspace":"%s","model":"%s"}\n' "$AGENT_ID" "$AGENT_WORKSPACE" "$AGENT_MODEL"
else
  CREATE_PARAMS="$(python3 - "$CREATE_NAME" "$AGENT_WORKSPACE" <<'PY'
import json, sys
print(json.dumps({"name": sys.argv[1], "workspace": sys.argv[2]}))
PY
)"
  gateway_call "agents.create" "$CREATE_PARAMS" >/dev/null
  if [[ -n "$AGENT_MODEL" || "$AGENT_NAME" != "$CREATE_NAME" ]]; then
    UPDATE_PARAMS="$(python3 - "$AGENT_ID" "$AGENT_NAME" "$AGENT_WORKSPACE" "$AGENT_MODEL" <<'PY'
import json, sys
payload = {
  "agentId": sys.argv[1],
  "name": sys.argv[2],
  "workspace": sys.argv[3],
}
if sys.argv[4]:
  payload["model"] = sys.argv[4]
print(json.dumps(payload))
PY
)"
    gateway_call "agents.update" "$UPDATE_PARAMS" >/dev/null
  fi
  printf '{"operation":"created","agent_id":"%s","workspace":"%s","model":"%s"}\n' "$AGENT_ID" "$AGENT_WORKSPACE" "$AGENT_MODEL"
fi
