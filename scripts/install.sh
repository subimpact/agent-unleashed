#!/usr/bin/env bash
#
# Agent-Unleashed (agt-ul) installer for macOS, Linux and Termux.
#
#   curl -fsSL https://agent.subimpact.net/install.sh | bash
#
# Downloads the published release binary, verifies its SHA-256 against the
# release checksum, and installs it on PATH. Falls back to building from source
# only when run inside a checkout. It never reports success without installing.

set -euo pipefail

REPO="subimpact/agent-unleashed"
INSTALL_DIR="${AGT_UL_INSTALL_DIR:-$HOME/.local/bin}"
BIN="$INSTALL_DIR/agt-ul"

step() { printf '\n%s\n' "$*"; }
ok()   { printf '  OK  %s\n' "$*"; }
fail() { printf '  FAILED  %s\n' "$*" >&2; exit 1; }

echo "============================================================"
echo "  Installing Agent-Unleashed (agt-ul)"
echo "============================================================"

mkdir -p "$INSTALL_DIR"

if [ -f "./main.go" ]; then
    step "[1/3] Local checkout detected - building from source..."
    command -v go >/dev/null 2>&1 || fail "Go toolchain not found. Install Go from https://go.dev/dl/ or run this script outside a checkout to download a release binary."
    go build -trimpath -ldflags "-s -w" -o "$BIN" . || fail "go build failed"
    ok "Compiled $BIN"
else
    case "$(uname -s)" in
        Linux)  os=linux  ;;
        Darwin) os=darwin ;;
        *)      fail "Unsupported OS: $(uname -s). Build from source with: go build -o agt-ul ." ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64)  arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *)             fail "Unsupported architecture: $(uname -m). Build from source with: go build -o agt-ul ." ;;
    esac

    asset="agt-ul_${os}_${arch}"
    base="https://github.com/$REPO/releases/latest/download"
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT

    step "[1/3] Downloading $asset ..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$base/$asset"       -o "$tmp/$asset"      || fail "Could not download $base/$asset"
        curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "Could not fetch checksums.txt - refusing to install an unverified binary."
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$tmp/$asset"       "$base/$asset"        || fail "Could not download $base/$asset"
        wget -qO "$tmp/checksums.txt" "$base/checksums.txt" || fail "Could not fetch checksums.txt - refusing to install an unverified binary."
    else
        fail "Neither curl nor wget is available."
    fi
    [ -s "$tmp/$asset" ] || fail "Downloaded file is empty."

    step "[2/3] Verifying checksum..."
    expected="$(awk -v a="$asset" '$2 == a || $2 == "*"a {print $1; exit}' "$tmp/checksums.txt")"
    [ -n "$expected" ] || fail "No checksum entry for $asset - refusing to install an unverified binary."

    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
    else
        fail "No sha256sum or shasum available - cannot verify the download."
    fi
    [ "$actual" = "$expected" ] || fail "Checksum mismatch. Expected $expected, got $actual."
    ok "SHA-256 verified"

    install -m 0755 "$tmp/$asset" "$BIN" 2>/dev/null || { cp "$tmp/$asset" "$BIN" && chmod 0755 "$BIN"; }
    ok "Installed $BIN"
fi

ln -sf "$BIN" "$INSTALL_DIR/agy-ul"

# PATH, added once per shell rc, and only for rc files that already exist.
step "[3/3] Ensuring $INSTALL_DIR is on PATH..."
line="export PATH=\"$INSTALL_DIR:\$PATH\""
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    added=""
    for rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.profile"; do
        [ -f "$rc" ] || continue
        if ! grep -qF "$line" "$rc" 2>/dev/null; then
            printf '\n# Added by the agent-unleashed installer\n%s\n' "$line" >> "$rc"
            added="$added $rc"
        fi
    done
    if [ -n "$added" ]; then
        ok "Added to:$added (restart your shell to pick it up)"
    else
        ok "No shell rc file to update - add this line yourself: $line"
    fi
    export PATH="$INSTALL_DIR:$PATH"
else
    ok "Already on PATH"
fi

[ -x "$BIN" ] || fail "Installation did not produce an executable at $BIN"

echo ""
echo "Installed: $("$BIN" version)"
echo "Run 'agt-ul setup' to configure, or 'agt-ul' to start."
