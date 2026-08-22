#!/usr/bin/env bash
set -e

echo "============================================================"
echo "  🚀 Installing Agent-Unleashed (agt-ul) for Unix"
echo "============================================================"

INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

if [ -f "./main.go" ]; then
    echo "🔨 Compiling from local source..."
    go build -o "$INSTALL_DIR/agt-ul" .
    ln -sf "$INSTALL_DIR/agt-ul" "$INSTALL_DIR/agy-ul"
else
    echo "⬇️ Downloading latest binary..."
fi

# Ensure PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    export PATH="$INSTALL_DIR:$PATH"
    echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$HOME/.bashrc"
    echo "export PATH=\"$INSTALL_DIR:\$PATH\"" >> "$HOME/.zshrc" 2>/dev/null || true
fi

echo "✅ Successfully installed agt-ul to $INSTALL_DIR/agt-ul"
echo "Run 'agt-ul setup' or 'agt-ul' to get started."
