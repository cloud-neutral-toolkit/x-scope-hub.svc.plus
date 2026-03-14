#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CODEX_RS_DIR="$ROOT_DIR/third_party/codex/codex-rs"

cd "$CODEX_RS_DIR"
cargo build -p codex-cli --bin codex

echo "Built Codex from source:"
echo "  $CODEX_RS_DIR/target/debug/codex"
