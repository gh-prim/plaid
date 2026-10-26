#!/bin/sh
# Debian postrm: $1 = "remove" | "purge" | "upgrade" | ...
# Data in /var/lib/plaid is never deleted automatically: a library is too precious.
set -e

case "$1" in
  purge)
    rm -rf /etc/plaid
    echo "PLAID config removed. Your data is still in /var/lib/plaid (delete it yourself if you want)."
    ;;
esac

if [ -d /run/systemd/system ]; then
  systemctl daemon-reload || true
fi
