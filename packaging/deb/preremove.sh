#!/bin/sh
# Debian prerm: $1 = "remove" | "upgrade" | "deconfigure" | "failed-upgrade".
# On upgrade the service keeps running; postinst restarts it with the new binary.
set -e

case "$1" in
  remove | deconfigure)
    if [ -d /run/systemd/system ]; then
      systemctl stop plaid.service || true
      systemctl disable plaid.service || true
    fi
    ;;
esac
