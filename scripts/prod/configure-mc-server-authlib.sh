#!/usr/bin/env bash
#
# Configure a Forge/NeoForge Minecraft server to verify players against the
# production backend via authlib-injector.

set -euo pipefail

SERVER_DIR="${SERVER_DIR:-$PWD}"
PUBLIC_URL="${PUBLIC_BASE_URL:-}"
JAR_NAME="${AUTHLIB_JAR_NAME:-authlib-injector-1.2.5.jar}"

usage() {
  cat <<'EOF'
Usage: scripts/prod/configure-mc-server-authlib.sh --server-dir /path/to/server --public-url https://launcher.example.com [options]

Options:
  --server-dir DIR   Minecraft server directory
  --public-url URL   Public backend URL
  --jar-name NAME    authlib-injector jar name in server dir
                    (default: authlib-injector-1.2.5.jar)
  -h, --help         Show this help

This updates:
  - user_jvm_args.txt
  - server.properties
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --server-dir)
      SERVER_DIR="${2:-}"
      shift
      ;;
    --public-url)
      PUBLIC_URL="${2:-}"
      shift
      ;;
    --jar-name)
      JAR_NAME="${2:-}"
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

if [[ -z "$PUBLIC_URL" || "$PUBLIC_URL" != https://* ]]; then
  echo "ERROR: --public-url must be a public HTTPS URL." >&2
  exit 2
fi

if [[ ! -d "$SERVER_DIR" ]]; then
  echo "ERROR: server directory not found: $SERVER_DIR" >&2
  exit 1
fi

if [[ ! -f "$SERVER_DIR/$JAR_NAME" ]]; then
  echo "ERROR: authlib jar not found: $SERVER_DIR/$JAR_NAME" >&2
  echo "Copy it into the server directory or pass --jar-name." >&2
  exit 1
fi

USER_JVM_ARGS="$SERVER_DIR/user_jvm_args.txt"
SERVER_PROPERTIES="$SERVER_DIR/server.properties"
AGENT_URL="${PUBLIC_URL%/}/api/v1/integrations/authlib/minecraft"
AGENT_LINE="-javaagent:$JAR_NAME=$AGENT_URL"

backup_file() {
  local file="$1"
  if [[ -f "$file" ]]; then
    cp "$file" "$file.$(date +%Y%m%d-%H%M%S).bak"
  fi
}

mkdir -p "$SERVER_DIR"
touch "$USER_JVM_ARGS"
backup_file "$USER_JVM_ARGS"

TMP="$(mktemp)"
awk -v agent="$AGENT_LINE" '
  BEGIN { done = 0 }
  /^-javaagent:.*authlib.*=/ {
    if (!done) {
      print agent
      done = 1
    }
    next
  }
  { print }
  END {
    if (!done) {
      print agent
    }
  }
' "$USER_JVM_ARGS" > "$TMP"
mv "$TMP" "$USER_JVM_ARGS"

touch "$SERVER_PROPERTIES"
backup_file "$SERVER_PROPERTIES"

set_property() {
  local key="$1" value="$2" file="$3"
  local tmp
  tmp="$(mktemp)"
  awk -v key="$key" -v value="$value" '
    BEGIN { done = 0 }
    $0 ~ "^" key "=" {
      print key "=" value
      done = 1
      next
    }
    { print }
    END {
      if (!done) {
        print key "=" value
      }
    }
  ' "$file" > "$tmp"
  mv "$tmp" "$file"
}

set_property "online-mode" "true" "$SERVER_PROPERTIES"
set_property "enforce-secure-profile" "false" "$SERVER_PROPERTIES"

echo "[mc] Configured authlib-injector:"
echo "     $AGENT_LINE"
echo "[mc] Updated:"
echo "     $USER_JVM_ARGS"
echo "     $SERVER_PROPERTIES"
echo ""
echo "Restart the Minecraft server and check authlib-injector.log for:"
echo "  API root: ${PUBLIC_URL%/}/api/v1/integrations/authlib/minecraft"
