#!/usr/bin/env bash
#
# Quick production health check for backend authlib endpoints and MC port.

set -euo pipefail

PUBLIC_URL="${PUBLIC_BASE_URL:-}"
MC_HOST="${MC_HOST:-127.0.0.1}"
MC_PORT="${MC_PORT:-25565}"

usage() {
  cat <<'EOF'
Usage: scripts/prod/health-check.sh --public-url https://launcher.example.com [options]

Options:
  --public-url URL   Public backend URL
  --mc-host HOST     Minecraft server host (default: 127.0.0.1)
  --mc-port PORT     Minecraft server port (default: 25565)
  -h, --help         Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --public-url)
      PUBLIC_URL="${2:-}"
      shift
      ;;
    --mc-host)
      MC_HOST="${2:-}"
      shift
      ;;
    --mc-port)
      MC_PORT="${2:-}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [[ -z "$PUBLIC_URL" ]]; then
  echo "ERROR: --public-url is required." >&2
  usage >&2
  exit 2
fi

PUBLIC_URL="${PUBLIC_URL%/}"

need_curl() {
  if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl not found." >&2
    exit 1
  fi
}

check_http() {
  local label="$1" url="$2"
  printf '[check] %-28s ' "$label"
  if curl -fsS --max-time 10 "$url" >/dev/null; then
    echo "OK"
  else
    echo "FAIL"
    return 1
  fi
}

check_port() {
  local host="$1" port="$2"
  printf '[check] %-28s ' "minecraft $host:$port"
  if (echo >"/dev/tcp/$host/$port") >/dev/null 2>&1; then
    echo "OK"
  else
    echo "FAIL"
    return 1
  fi
}

need_curl

FAIL=0
check_http "yggdrasil meta" "$PUBLIC_URL/api/yggdrasil/" || FAIL=1
check_http "gml-compatible meta" "$PUBLIC_URL/api/v1/integrations/authlib/minecraft/" || FAIL=1
check_http "authlib jar" "$PUBLIC_URL/api/yggdrasil/authlib-injector.jar" || FAIL=1
check_port "$MC_HOST" "$MC_PORT" || FAIL=1

if (( FAIL == 0 )); then
  echo "[check] Production stack looks reachable."
else
  echo "[check] Some checks failed."
fi

exit "$FAIL"
