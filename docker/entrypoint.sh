#!/bin/bash
set -euo pipefail

# maildump の保存先を用意する。
# compose で ./logs をマウントした場合に備え、権限も整える。
MAILDIR="${MAILDUMP_DIR:-/var/log/maildump}"
mkdir -p "$MAILDIR"
chown -R www-data:www-data "$MAILDIR" 2>/dev/null || true

exec "$@"
