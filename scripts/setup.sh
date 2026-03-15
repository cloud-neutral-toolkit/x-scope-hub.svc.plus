#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  setup.sh <repo_name_or_dir> [--repo <git_url>] [--ref <git_ref>] [--dir <path>] [--mode <process|docker|cloud-run>] [--action <install|test|uninstall>]

Examples:
  # Remote install:
  # curl -fsSL "https://raw.githubusercontent.com/cloud-neutral-toolkit/x-scope-hub.svc.plus/main/scripts/setup.sh?$(date +%s)" | bash -s -- x-scope-hub.svc.plus --mode process
  #
  # Local:
  # bash scripts/setup.sh x-scope-hub.svc.plus --mode docker

Notes:
  - Default action is install.
  - Process and docker modes write the default Caddy site config to /etc/caddy/conf.d/<domain>.conf.
  - If .env does not exist, it copies .env.example -> .env (placeholder only).
EOF
}

log() { printf '[setup] %s\n' "$*"; }

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log "missing required command: $1"
    exit 1
  fi
}

if [[ "${1:-}" == "" || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

NAME="$1"
shift

REPO_URL=""
REF="main"
DIR="$NAME"
MODE="process"
ACTION="install"
APP_PORT="${XSCOPE_APP_PORT:-18085}"
SERVICE_NAME="x-scope-hub"
IMAGE_NAME="x-scope-hub-mcp:local"
CONTAINER_NAME="x-scope-hub"
SYSTEMD_UNIT="x-scope-hub.service"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO_URL="${2:-}"; shift 2 ;;
    --ref) REF="${2:-}"; shift 2 ;;
    --dir) DIR="${2:-}"; shift 2 ;;
    --mode) MODE="${2:-}"; shift 2 ;;
    --action) ACTION="${2:-}"; shift 2 ;;
    *) log "unknown arg: $1"; usage; exit 2 ;;
  esac
done

case "${MODE}" in
  process|docker|cloud-run) ;;
  *)
    log "invalid mode: ${MODE}"
    usage
    exit 2
    ;;
esac

case "${ACTION}" in
  install|test|uninstall) ;;
  *)
    log "invalid action: ${ACTION}"
    usage
    exit 2
    ;;
esac

if [[ -z "${REPO_URL}" ]]; then
  REPO_URL="https://github.com/cloud-neutral-toolkit/${NAME}.git"
fi

need_cmd git
need_cmd curl

DOMAIN="$(basename "${NAME}")"
CADDY_DIR="/etc/caddy/conf.d"
CADDY_CONF="${CADDY_DIR}/${DOMAIN}.conf"

require_root_for_local_install() {
  if [[ "${MODE}" == "process" || "${MODE}" == "docker" ]]; then
    if [[ "$(id -u)" -ne 0 ]]; then
      log "action=${ACTION} mode=${MODE} requires root to manage systemd/docker and /etc/caddy/conf.d"
      exit 1
    fi
  fi
}

ensure_repo() {
  if [[ -e "${DIR}" && ! -d "${DIR}" ]]; then
    log "path exists and is not a directory: ${DIR}"
    exit 2
  fi

  if [[ ! -d "${DIR}" ]]; then
    log "cloning ${REPO_URL} (ref=${REF}) -> ${DIR}"
    git clone --depth 1 --branch "${REF}" "${REPO_URL}" "${DIR}"
    return
  fi

  if [[ ! -d "${DIR}/.git" ]]; then
    log "directory exists but is not a git repo: ${DIR}"
    exit 2
  fi

  log "repo directory already exists: ${DIR}"
}

sync_repo_ref() {
  log "syncing repository to ref=${REF}"
  git fetch --depth 1 origin "${REF}"
  git checkout -f FETCH_HEAD
}

download_go_modules() {
  local did_any="false"

  if command -v go >/dev/null 2>&1; then
    log "detected Go toolchain: $(go version)"
    if [[ -f "observe-bridge/go.mod" ]]; then
      log "downloading observe-bridge dependencies"
      (cd observe-bridge && go mod download)
      did_any="true"
    fi
    if [[ -f "observe-gateway/go.mod" ]]; then
      log "downloading observe-gateway dependencies"
      (cd observe-gateway && go mod download)
      did_any="true"
    fi
    if [[ -f "llm-ops-agent/go.mod" ]]; then
      log "downloading llm-ops-agent dependencies"
      (cd llm-ops-agent && go mod download)
      did_any="true"
    fi
    if [[ -f "mcp-server/go.mod" ]]; then
      log "downloading mcp-server dependencies"
      (cd mcp-server && go mod download)
      did_any="true"
    fi
  elif [[ "${MODE}" == "process" ]]; then
    log "missing required command for process mode: go"
    exit 1
  else
    log "go not found; skipping local Go dependency download for mode=${MODE}"
  fi

  if [[ "${did_any}" == "false" ]]; then
    log "no supported Go modules detected."
  fi
}

ensure_env_file() {
  if [[ ! -f ".env" && -f ".env.example" ]]; then
    log "creating .env from .env.example (placeholder only)"
    cp .env.example .env
  fi
}

write_caddy_conf() {
  mkdir -p "${CADDY_DIR}"
  cat > "${CADDY_CONF}" <<EOF
${DOMAIN} {
    encode gzip zstd
    reverse_proxy 127.0.0.1:${APP_PORT}
}
EOF
  log "wrote Caddy site config: ${CADDY_CONF}"
  if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files caddy.service >/dev/null 2>&1; then
    systemctl reload caddy || systemctl restart caddy
  fi
}

remove_caddy_conf() {
  if [[ -f "${CADDY_CONF}" ]]; then
    rm -f "${CADDY_CONF}"
    log "removed Caddy site config: ${CADDY_CONF}"
    if command -v systemctl >/dev/null 2>&1 && systemctl list-unit-files caddy.service >/dev/null 2>&1; then
      systemctl reload caddy || systemctl restart caddy
    fi
  fi
}

install_process_mode() {
  need_cmd go
  need_cmd systemctl
  log "building mcp-server binary"
  (cd mcp-server && go build -o mcp ./cmd/mcp)
  cat > "/etc/systemd/system/${SYSTEMD_UNIT}" <<EOF
[Unit]
Description=XScopeHub MCP Server
After=network.target

[Service]
Type=simple
WorkingDirectory=$(pwd)/mcp-server
EnvironmentFile=$(pwd)/.env
ExecStart=$(pwd)/mcp-server/mcp serve -addr 127.0.0.1:${APP_PORT} -manifest $(pwd)/mcp-server/manifest.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now "${SYSTEMD_UNIT}"
  write_caddy_conf
}

uninstall_process_mode() {
  if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now "${SYSTEMD_UNIT}" >/dev/null 2>&1 || true
    rm -f "/etc/systemd/system/${SYSTEMD_UNIT}"
    systemctl daemon-reload || true
    systemctl reset-failed || true
  fi
  remove_caddy_conf
}

install_docker_mode() {
  need_cmd docker
  log "building docker image ${IMAGE_NAME}"
  docker build -t "${IMAGE_NAME}" -f mcp-server/Dockerfile .
  docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
  docker run -d \
    --name "${CONTAINER_NAME}" \
    --restart unless-stopped \
    --env-file .env \
    -e PORT=8080 \
    -p "127.0.0.1:${APP_PORT}:8080" \
    "${IMAGE_NAME}"
  write_caddy_conf
}

uninstall_docker_mode() {
  if command -v docker >/dev/null 2>&1; then
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    docker image rm "${IMAGE_NAME}" >/dev/null 2>&1 || true
  fi
  remove_caddy_conf
}

run_tests() {
  local failed=0
  if ! curl -fsS "http://127.0.0.1:${APP_PORT}/manifest" >/dev/null; then
    log "local manifest check failed on 127.0.0.1:${APP_PORT}"
    failed=1
  else
    log "local manifest check passed on 127.0.0.1:${APP_PORT}"
  fi

  if [[ "${MODE}" == "process" ]] && command -v systemctl >/dev/null 2>&1; then
    if systemctl is-active --quiet "${SYSTEMD_UNIT}"; then
      log "systemd unit ${SYSTEMD_UNIT} is active"
    else
      log "systemd unit ${SYSTEMD_UNIT} is not active"
      failed=1
    fi
  fi

  if [[ "${MODE}" == "docker" ]] && command -v docker >/dev/null 2>&1; then
    if docker ps --format '{{.Names}}' | grep -qx "${CONTAINER_NAME}"; then
      log "docker container ${CONTAINER_NAME} is running"
    else
      log "docker container ${CONTAINER_NAME} is not running"
      failed=1
    fi
  fi

  if [[ -f "${CADDY_CONF}" ]]; then
    log "caddy config present: ${CADDY_CONF}"
  else
    log "caddy config missing: ${CADDY_CONF}"
    failed=1
  fi

  if [[ "${failed}" -ne 0 ]]; then
    exit 1
  fi
}

print_next_steps() {
  log "setup complete"
  log "deployment mode: ${MODE}"
  log "action: ${ACTION}"
  log "next steps:"
  case "${MODE}" in
    process)
      log "  systemctl status ${SYSTEMD_UNIT}"
      log "  curl http://127.0.0.1:${APP_PORT}/manifest"
      ;;
    docker)
      log "  docker ps | grep ${CONTAINER_NAME}"
      log "  curl http://127.0.0.1:${APP_PORT}/manifest"
      ;;
    cloud-run)
      log "  gcloud auth login"
      log "  gcloud config set project <your-gcp-project>"
      log "  gcloud run deploy xscopehub-mcp --source ./mcp-server --region <your-region> --allow-unauthenticated"
      ;;
  esac
}

require_root_for_local_install

if [[ "${ACTION}" == "install" ]]; then
  ensure_repo
  cd "${DIR}"
  sync_repo_ref
  download_go_modules
  ensure_env_file
  if [[ -f "scripts/post-setup.sh" ]]; then
    log "running scripts/post-setup.sh"
    bash scripts/post-setup.sh
  fi
fi

cd "${DIR}" 2>/dev/null || true

case "${ACTION}:${MODE}" in
  install:process) install_process_mode ;;
  install:docker) install_docker_mode ;;
  install:cloud-run) print_next_steps; exit 0 ;;
  test:process|test:docker) run_tests ;;
  uninstall:process) uninstall_process_mode ;;
  uninstall:docker) uninstall_docker_mode ;;
  uninstall:cloud-run) log "nothing to uninstall for cloud-run mode in local host script" ;;
  test:cloud-run) log "cloud-run mode test is not handled by the local host script"; exit 2 ;;
esac

print_next_steps
