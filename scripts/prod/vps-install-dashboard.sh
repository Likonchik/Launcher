#!/usr/bin/env bash
#
# Build and install the admin dashboard as a systemd service on a VPS.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DASHBOARD_DIR="$ROOT_DIR/dashboard"
SERVICE_NAME="${SERVICE_NAME:-project-minecraft-dashboard}"
SERVICE_USER="${SUDO_USER:-$USER}"
API_URL="${NEXT_PUBLIC_API_URL:-}"
INSTALL_SERVICE=1

usage() {
  cat <<'EOF'
Usage: scripts/prod/vps-install-dashboard.sh --api-url https://launcher.example.com [options]

Options:
  --api-url URL       Public backend URL used by dashboard
  --service-name NAME systemd service name
  --no-service        Build only, do not install systemd service
  -h, --help          Show this help

The service listens on 127.0.0.1:3000. Put it behind Nginx/Caddy if you want
external access to the admin dashboard.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --api-url)
      API_URL="${2:-}"
      shift
      ;;
    --service-name)
      SERVICE_NAME="${2:-}"
      shift
      ;;
    --no-service)
      INSTALL_SERVICE=0
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

if [[ -z "$API_URL" || "$API_URL" != https://* ]]; then
  echo "ERROR: --api-url must be a public HTTPS URL, for example https://launcher.example.com" >&2
  exit 2
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "ERROR: npm not found. Install Node.js/npm on the VPS." >&2
  exit 1
fi

cat > "$DASHBOARD_DIR/.env.production" <<EOF
NEXT_PUBLIC_API_URL=${API_URL%/}
EOF

echo "[dashboard] Installing dependencies"
npm --prefix "$DASHBOARD_DIR" install

echo "[dashboard] Building dashboard"
(
  cd "$DASHBOARD_DIR"
  NEXT_PUBLIC_API_URL="${API_URL%/}" npm run build
)

if (( INSTALL_SERVICE == 0 )); then
  echo "[dashboard] Built successfully."
  exit 0
fi

if ! command -v sudo >/dev/null 2>&1; then
  echo "ERROR: sudo not found. Re-run with --no-service or install systemd service manually." >&2
  exit 1
fi

NPM_BIN="$(command -v npm)"
UNIT_PATH="/etc/systemd/system/$SERVICE_NAME.service"
TMP_UNIT="$(mktemp)"
cat > "$TMP_UNIT" <<EOF
[Unit]
Description=Project Minecraft Admin Dashboard
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$SERVICE_USER
WorkingDirectory=$DASHBOARD_DIR
Environment=NODE_ENV=production
EnvironmentFile=$DASHBOARD_DIR/.env.production
ExecStart=$NPM_BIN run start
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

echo "[dashboard] Installing systemd unit: $UNIT_PATH"
sudo install -m 0644 "$TMP_UNIT" "$UNIT_PATH"
rm -f "$TMP_UNIT"
sudo systemctl daemon-reload
sudo systemctl enable --now "$SERVICE_NAME"
sudo systemctl restart "$SERVICE_NAME"

echo ""
echo "[dashboard] Installed and started."
sudo systemctl --no-pager --full status "$SERVICE_NAME" || true
