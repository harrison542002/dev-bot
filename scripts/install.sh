#!/usr/bin/env bash
set -euo pipefail

# ─── config ───────────────────────────────────────────────────────────────────
REPO="harrison542002/dev-bot"
BINARY="devbot"
INSTALL_DIR="${DEVBOT_INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${DEVBOT_CONFIG_DIR:-$HOME/.config/devbot}"
# ──────────────────────────────────────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { echo -e "${CYAN}${BOLD}==>${RESET} $*"; }
success() { echo -e "${GREEN}${BOLD}✓${RESET}  $*"; }
warn()    { echo -e "${YELLOW}${BOLD}!${RESET}  $*"; }
die()     { echo -e "${RED}${BOLD}✗${RESET}  $*" >&2; exit 1; }

# ─── platform detection ───────────────────────────────────────────────────────
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Linux)   os="linux"   ;;
        Darwin)  os="darwin"  ;;
        MINGW*|MSYS*|CYGWIN*) os="windows" ;;
        *) die "Unsupported OS: $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) die "Unsupported architecture: $(uname -m)" ;;
    esac

    echo "${os}_${arch}"
}

# ─── dependency checks ────────────────────────────────────────────────────────
need_cmd() {
    command -v "$1" &>/dev/null || die "Required command not found: $1 — please install it and retry."
}

need_cmd curl
need_cmd tar
need_cmd git   # used by the agent at runtime — warn if missing

# ─── resolve latest version ───────────────────────────────────────────────────
info "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

VERSION="${DEVBOT_VERSION:-$LATEST}"
[ -n "$VERSION" ] || die "Could not determine latest version. Set DEVBOT_VERSION to override."
info "Version: ${BOLD}${VERSION}${RESET}"

# ─── download ─────────────────────────────────────────────────────────────────
PLATFORM=$(detect_platform)
TARBALL="${BINARY}_${VERSION}_${PLATFORM}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

info "Downloading ${TARBALL}..."
curl -fsSL --progress-bar "$URL" -o "${TMP}/${TARBALL}" \
    || die "Download failed. Check that ${VERSION} has a release for ${PLATFORM}.\n  URL: ${URL}"

info "Extracting..."
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"

# ─── install binary ───────────────────────────────────────────────────────────
BINARY_PATH="${TMP}/${BINARY}"
[ -f "$BINARY_PATH" ] || BINARY_PATH="${TMP}/${BINARY}.exe"   # Windows fallback
[ -f "$BINARY_PATH" ] || die "Binary not found in archive."
chmod +x "$BINARY_PATH"

if [ -w "$INSTALL_DIR" ]; then
    mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"
else
    info "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$BINARY_PATH" "${INSTALL_DIR}/${BINARY}"
fi

success "Installed ${BOLD}${BINARY}${RESET} → ${INSTALL_DIR}/${BINARY}"

# ─── config scaffold ──────────────────────────────────────────────────────────
if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
    mkdir -p "$CONFIG_DIR"

    cat > "${CONFIG_DIR}/config.yaml" <<'EOF'
bot:
  platform: "telegram"   # or "discord"

telegram:
  token: ""              # from @BotFather
  allowed_user_ids: []   # your Telegram user ID

git:
  name: "DevBot"
  email: "devbot@users.noreply.github.com"

github:
  token: ""              # GitHub PAT (Contents + Pull requests Read & Write)
  owner: ""
  repo: ""
  base_branch: "main"

ai:
  provider: "local"      # claude | openai | gemini | local

local:
  base_url: "http://localhost:11434"
  model: "gemma4"

database:
  path: "./devbot.db"

schedule:
  enabled: false
  timezone: "UTC"
  work_start: "09:00"
  work_end: "17:00"
  check_interval_minutes: 10
  enable_weekend: false
EOF

    chmod 600 "${CONFIG_DIR}/config.yaml"
    success "Config scaffold written to ${CONFIG_DIR}/config.yaml"
    warn "Edit ${CONFIG_DIR}/config.yaml and fill in your tokens before running devbot."
else
    info "Config already exists at ${CONFIG_DIR}/config.yaml — skipping scaffold."
fi

# ─── PATH hint ────────────────────────────────────────────────────────────────
if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    warn "${INSTALL_DIR} is not in your PATH."
    warn "Add this to your shell profile:"
    echo -e "    ${BOLD}export PATH=\"${INSTALL_DIR}:\$PATH\"${RESET}"
fi

# ─── done ─────────────────────────────────────────────────────────────────────
echo
success "DevBot ${VERSION} installed successfully!"
echo
echo -e "  Edit config:  ${BOLD}${CONFIG_DIR}/config.yaml${RESET}"
echo -e "  Run:          ${BOLD}devbot -config ${CONFIG_DIR}/config.yaml${RESET}"
echo
