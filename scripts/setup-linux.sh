#!/usr/bin/env bash
# Prepares a Debian/Ubuntu machine to run the Speed News desktop app natively (tray included).
# For daily development without any of this, open the project in the Dev Container instead.
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

print_step() { printf '\n\033[1;34m==> %s\033[0m\n' "$1"; }

if ! command -v apt-get >/dev/null; then
  echo "This script supports Debian/Ubuntu (apt). See https://v2.tauri.app/start/prerequisites/ for other distros."
  exit 1
fi

print_step "Tauri system libraries (sudo)"
sudo apt-get update
sudo apt-get install -y build-essential curl wget file pkg-config \
  libwebkit2gtk-4.1-dev libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev

print_step "Rust (rustup; the exact version comes from rust-toolchain.toml)"
if ! command -v rustup >/dev/null; then
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain none
  # shellcheck source=/dev/null
  source "${HOME}/.cargo/env"
fi
# Without arguments, `rustup toolchain install` installs the version from rust-toolchain.toml.
(cd "${repository_root}/apps/desktop/src-tauri" && rustup toolchain install && rustup show active-toolchain)

print_step "Go and Node (mise, versions from .mise.toml)"
if ! command -v mise >/dev/null; then
  curl -fsSL https://mise.run | sh
  export PATH="${HOME}/.local/bin:${PATH}"
  echo "mise installed. Add 'eval \"\$(mise activate bash)\"' to your ~/.bashrc to use it in new shells."
fi
(cd "${repository_root}" && mise trust --yes && mise install)

print_step "JavaScript dependencies"
(cd "${repository_root}" && mise exec -- npm ci)

if [[ "${XDG_CURRENT_DESKTOP:-}" == *GNOME* ]] && ! gnome-extensions list 2>/dev/null | grep -qi appindicator; then
  echo
  echo "⚠ GNOME without the AppIndicator extension: the tray icon will not appear."
  echo "  Install it with: sudo apt install gnome-shell-extension-appindicator (then log out and back in)"
fi

print_step "Done"
echo "Run: make dev   (the Makefile finds Rust and the mise tools by itself)"
echo "To use cargo/go/node directly in this terminal, run: source ~/.cargo/env && eval \"\$(~/.local/bin/mise activate bash)\""
