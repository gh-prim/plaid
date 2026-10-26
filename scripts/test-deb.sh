#!/usr/bin/env bash
# Tests the PLAID .deb inside Docker with a real systemd (PID 1): install,
# service, upgrade with edited config, install script, tampered package,
# restart (reboot-like), remove, purge, plus cold start and idle memory.
#
# Usage: scripts/test-deb.sh [image] [arch]
#   scripts/test-deb.sh ubuntu:24.04 arm64
#   scripts/test-deb.sh debian:bookworm amd64
# Env: PLAID_TEST_CPUS (e.g. 0.5) and PLAID_TEST_MEMORY (e.g. 1g) to throttle
# the container and approximate small hardware. Needs `make deb` first.
set -euo pipefail

image="${1:-ubuntu:24.04}"
case "${2:-$(uname -m)}" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) echo "unsupported arch: ${2:-$(uname -m)}" >&2; exit 2 ;;
esac

root="$(cd "$(dirname "$0")/.." && pwd)"
deb="$(find "$root/dist" -maxdepth 1 -name "plaid_*_${arch}.deb" -exec basename {} \; 2>/dev/null | head -n1)"
[ -n "$deb" ] || { echo "no $arch package in dist/, run 'make deb' first" >&2; exit 2; }

tag="plaid-systemd-test:$(echo "$image" | tr ':/' '--')-$arch"
name="plaid-test-$$"

docker build -q --platform "linux/$arch" -t "$tag" - >/dev/null <<DOCKERFILE
FROM $image
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus curl ca-certificates adduser procps \
 && rm -rf /var/lib/apt/lists/*
STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
DOCKERFILE

limits=()
[ -n "${PLAID_TEST_CPUS:-}" ] && limits+=(--cpus "$PLAID_TEST_CPUS")
[ -n "${PLAID_TEST_MEMORY:-}" ] && limits+=(--memory "$PLAID_TEST_MEMORY")

docker run -d --name "$name" --platform "linux/$arch" --privileged --cgroupns=host \
  -v /sys/fs/cgroup:/sys/fs/cgroup:rw --tmpfs /run --tmpfs /run/lock \
  ${limits[@]+"${limits[@]}"} "$tag" >/dev/null
trap 'docker rm -f "$name" >/dev/null 2>&1' EXIT

docker exec "$name" mkdir -p /pkg
docker cp "$root/dist/$deb" "$name:/pkg/"
docker cp "$root/dist/checksums.txt" "$name:/pkg/"
docker cp "$root/packaging/install.sh" "$name:/pkg/"

# Shared helpers, prepended to each phase run inside the container.
# shellcheck disable=SC2016 # expanded inside the container, not here
helpers='
set -u
fail=0
check() { local d="$1"; shift; if "$@" >/dev/null 2>&1; then echo "  ✅ $d"; else echo "  ❌ $d"; fail=1; fi; }
healthy() { for _ in $(seq 1 100); do curl -fsS localhost:32500/api/v1/health >/dev/null 2>&1 && return 0; sleep 0.1; done; return 1; }
not_active() { ! systemctl is-active --quiet plaid; }
'

phase() { docker exec -i -e DEB="$deb" "$name" bash -s; }

echo "== $image ($arch) — $deb"
docker exec "$name" systemctl is-system-running --wait >/dev/null 2>&1 || true

phase <<PHASE1 || status=1
$helpers
cd /pkg
check "install with apt" apt-get install -y "./\$DEB"
check "service active" systemctl is-active plaid
check "service enabled on boot" systemctl is-enabled plaid
check "health endpoint ok" healthy
check "web app served" bash -c 'curl -fsS localhost:32500/ | grep -q "<div id=\"root\">"'
check "runs as plaid" test "\$(ps -o user= -C plaid)" = plaid
check "database created" test -f /var/lib/plaid/plaid.db
check "config readable by plaid only" bash -c 'test "\$(stat -c %U:%G:%a /etc/plaid/plaid.env)" = root:plaid:640'
sed -i 's/^PLAID_LOG_LEVEL=info/PLAID_LOG_LEVEL=debug/' /etc/plaid/plaid.env
check "reinstall (upgrade path)" apt-get install --reinstall -y "./\$DEB"
check "edited config kept on upgrade" grep -q '^PLAID_LOG_LEVEL=debug' /etc/plaid/plaid.env
check "service back after upgrade" healthy
check "install script: valid package" env PLAID_DEB_URL="file:///pkg/\$DEB" PLAID_CHECKSUMS_URL=file:///pkg/checksums.txt bash install.sh
cp "\$DEB" /tmp/tampered.deb && echo x >> /tmp/tampered.deb
echo "\$(sha256sum "\$DEB" | cut -d' ' -f1)  tampered.deb" > /tmp/checksums.txt
check "install script: tampered package refused" bash -c '! PLAID_DEB_URL=file:///tmp/tampered.deb PLAID_CHECKSUMS_URL=file:///tmp/checksums.txt bash install.sh'
check "hardening exposure below 2.0" bash -c 'systemd-analyze security plaid 2>/dev/null | tail -1 | grep -Eq "level for plaid.service: (0|1)\.[0-9]"'
exit \$fail
PHASE1

docker restart "$name" >/dev/null
docker exec "$name" systemctl is-system-running --wait >/dev/null 2>&1 || true

phase <<PHASE2 || status=1
$helpers
check "service back after restart (reboot-like)" healthy
systemctl stop plaid
start=\$(date +%s%N); systemctl start plaid; healthy
echo "  ⏱  cold start to healthy: \$(( (\$(date +%s%N) - start) / 1000000 )) ms"
sleep 3
echo "  🧠 idle RSS: \$(( \$(ps -o rss= -C plaid) / 1024 )) MB"
check "remove" apt-get remove -y plaid
check "service stopped after remove" not_active
check "data kept after remove" test -f /var/lib/plaid/plaid.db
check "config kept after remove" test -f /etc/plaid/plaid.env
check "purge" apt-get purge -y plaid
check "config removed after purge" test ! -e /etc/plaid
check "data still kept after purge" test -f /var/lib/plaid/plaid.db
exit \$fail
PHASE2

if [ "${status:-0}" -ne 0 ]; then
  echo "❌ $image ($arch): some checks failed" >&2
  exit 1
fi
echo "✅ $image ($arch): all checks passed"
