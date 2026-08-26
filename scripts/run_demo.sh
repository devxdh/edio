#!/usr/bin/env bash
set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "=================================================="
echo " Building edio binary & populating demo repository"
echo "=================================================="

go build -o bin/edio ./cmd/edio
"$REPO_ROOT/scripts/setup_demo_repo.sh"

echo ""
echo "=================================================="
echo " Launching edio tview Split-Pane TUI Dashboard..."
echo " Use [j/k/↑/↓] or Mouse to navigate turns,"
echo " [Tab/h/l] to switch pane, [r] to revert, [q] to quit."
echo "=================================================="
echo ""

cd /tmp/edio-demo-session
exec "$REPO_ROOT/bin/edio" tui
