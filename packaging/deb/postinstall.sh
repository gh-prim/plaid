#!/bin/sh
# Debian postinst: $1 = "configure", $2 = previously installed version (empty on first install).
set -e

if ! getent group plaid >/dev/null; then
  addgroup --system plaid
fi
if ! getent passwd plaid >/dev/null; then
  adduser --system --ingroup plaid --home /var/lib/plaid --no-create-home \
    --shell /usr/sbin/nologin --gecos "PLAID media server" plaid
fi
# GPU access for hardware transcoding (F006), only where the groups exist.
for group in video render; do
  if getent group "$group" >/dev/null; then
    usermod -aG "$group" plaid
  fi
done

mkdir -p /var/lib/plaid
chown plaid:plaid /var/lib/plaid
chmod 750 /var/lib/plaid
chown root:plaid /etc/plaid/plaid.env
chmod 640 /etc/plaid/plaid.env

if [ -d /run/systemd/system ]; then
  systemctl daemon-reload
  if [ -n "${2:-}" ]; then
    systemctl restart plaid.service
  else
    systemctl enable --now plaid.service
    echo "PLAID is running: open http://$(hostname -I 2>/dev/null | cut -d' ' -f1):32500"
  fi
fi
