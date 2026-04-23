#!/usr/bin/env bash
set -euo pipefail

REPO="harrison542002/dev-bot"
BINARY="devbot"
DEFAULT_UNIX_INSTALL_DIR="/usr/local/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()    { echo -e "${CYAN}${BOLD}==>${RESET} $*"; }
success() { echo -e "${GREEN}${BOLD}OK${RESET}  $*"; }
warn()    { echo -e "${YELLOW}${BOLD}!${RESET}  $*"; }
die()     { echo -e "${RED}${BOLD}X${RESET}  $*" >&2; exit 1; }

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

is_wsl() {
    [ -n "${WSL_INTEROP:-}" ] || grep -qi microsoft /proc/version 2>/dev/null
}

detect_os() {
    case "$(uname -s)" in
        Linux)
            if is_wsl; then
                warn "Detected WSL. This script installs the Linux build inside WSL."
                warn "For native Windows installation, run scripts/install.ps1 from PowerShell."
            fi
            echo "linux"
            ;;
        Darwin)
            echo "darwin"
            ;;
        MINGW*|MSYS*|CYGWIN*)
            echo "windows"
            ;;
        *)
            die "Unsupported OS: $(uname -s)"
            ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            die "Unsupported architecture: $(uname -m)"
            ;;
    esac
}

fetch_latest_version() {
    curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' \
        | head -1 \
        | sed 's/.*"tag_name": *"\(.*\)".*/\1/' \
        | tr -d '\r'
}

extract_archive() {
    local archive_path="$1"
    local os="$2"
    local dest="$3"

    if [ "$os" = "windows" ]; then
        unzip -q "$archive_path" -d "$dest"
    else
        tar -xzf "$archive_path" -C "$dest"
    fi
}

install_binary() {
    local source_path="$1"
    local target_path="$2"
    local install_dir="$3"
    local os="$4"

    mkdir -p "$install_dir"

    if [ "$os" != "windows" ]; then
        chmod +x "$source_path"
    fi

    if [ "$os" = "windows" ] || [ -w "$install_dir" ]; then
        mv "$source_path" "$target_path"
        return
    fi

    info "Installing to ${install_dir} (requires sudo)..."
    sudo mv "$source_path" "$target_path"
}

print_path_hint() {
    local install_dir="$1"
    local os="$2"

    case ":$PATH:" in
        *":${install_dir}:"*) return ;;
    esac

    warn "${install_dir} is not in your PATH."
    if [ "$os" = "windows" ]; then
        warn "Add it to your Git Bash or terminal PATH before running devbot."
    else
        warn "Add this to your shell profile:"
        echo -e "    ${BOLD}export PATH=\"${install_dir}:\$PATH\"${RESET}"
    fi
}

main() {
    local os arch version archive_name archive_url archive_path tmp_dir
    local install_dir binary_name_in_archive install_target

    need_cmd curl
    need_cmd git

    os="$(detect_os)"
    arch="$(detect_arch)"

    if [ "$os" = "windows" ]; then
        need_cmd unzip
        install_dir="${DEVBOT_INSTALL_DIR:-$HOME/bin}"
        archive_name="${BINARY}-${os}-${arch}.zip"
        binary_name_in_archive="${BINARY}-${os}-${arch}.exe"
        install_target="${install_dir}/${BINARY}.exe"
    else
        need_cmd tar
        install_dir="${DEVBOT_INSTALL_DIR:-$DEFAULT_UNIX_INSTALL_DIR}"
        archive_name="${BINARY}-${os}-${arch}.tar.gz"
        binary_name_in_archive="${BINARY}-${os}-${arch}"
        install_target="${install_dir}/${BINARY}"
    fi

    info "Fetching latest release..."
    version="${DEVBOT_VERSION:-$(fetch_latest_version)}"
    [ -n "$version" ] || die "Could not determine latest version. Set DEVBOT_VERSION to override."
    info "Version: ${BOLD}${version}${RESET}"

    archive_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
    tmp_dir="$(mktemp -d)"
    trap 'rm -rf "$tmp_dir"' EXIT
    archive_path="${tmp_dir}/${archive_name}"

    info "Downloading ${archive_name}..."
    curl -fsSL --progress-bar "$archive_url" -o "$archive_path" \
        || die "Download failed. Check that ${version} has a release for ${os}/${arch}.\n  URL: ${archive_url}"

    info "Extracting..."
    extract_archive "$archive_path" "$os" "$tmp_dir"

    [ -f "${tmp_dir}/${binary_name_in_archive}" ] || die "Binary not found in archive: ${binary_name_in_archive}"

    install_binary "${tmp_dir}/${binary_name_in_archive}" "$install_target" "$install_dir" "$os"
    success "Installed ${BOLD}${BINARY}${RESET} -> ${install_target}"

    print_path_hint "$install_dir" "$os"

    echo
    success "DevBot ${version} installed successfully!"
    echo
    if [ "$os" = "windows" ]; then
        echo -e "  Run:          ${BOLD}${install_target}${RESET}"
    else
        echo -e "  Run:          ${BOLD}devbot${RESET}"
    fi
    echo
}

main "$@"
