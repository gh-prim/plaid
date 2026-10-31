#!/usr/bin/env bash
# Builds PLAID from this working tree and replaces the binary of a server
# installed from the .deb, then restarts it. sudo asks for the password on
# the server, in your terminal, unless SUDO_PASSWORD is set: then it goes to
# sudo through stdin, never on a command line.
#
#   scripts/deploy.sh                       # administrator@192.168.1.93
#   scripts/deploy.sh user@host [amd64|arm64]
#   SUDO_PASSWORD="$(op read op://Private/srv-plex-prod/password)" scripts/deploy.sh
#
# Like the releases, TMDB_API_KEY and FANART_API_KEY (environment) become
# the built-in keys; without them the server needs its own key in the admin.
set -euo pipefail

target="${1:-administrator@192.168.1.93}"
arch="${2:-amd64}"
root="$(cd "$(dirname "$0")/.." && pwd)"
version="0.1.19+local.$(date +%m%d%H%M)"
commit="$(git -C "$root" rev-parse --short HEAD)$(git -C "$root" diff --quiet || echo -dirty)"
bin="$root/server/bin/plaid-linux-$arch"

echo "▸ building the web app"
make -C "$root" web-build >/dev/null

echo "▸ building plaid $version ($commit) for linux/$arch"
CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go -C "$root/server" build -trimpath \
  -ldflags "-s -w -X github.com/gh-prim/plaid/server/internal/buildinfo.Version=$version -X github.com/gh-prim/plaid/server/internal/buildinfo.Commit=$commit -X github.com/gh-prim/plaid/server/internal/metadata.builtinTMDBKey=${TMDB_API_KEY:-} -X github.com/gh-prim/plaid/server/internal/metadata.builtinFanartKey=${FANART_API_KEY:-}" \
  -o "$bin" ./cmd/plaid

[ -n "${TMDB_API_KEY:-}" ] || echo "⚠ no TMDB_API_KEY: this build has no built-in key (set one in Admin ▸ Metadata)"

echo "▸ copying to $target"
scp -q "$bin" "$target:/tmp/plaid"

echo "▸ installing and restarting (sudo on the server)"
install='install -m 0755 /tmp/plaid /usr/bin/plaid && systemctl restart plaid'
after='rm /tmp/plaid && sleep 2 && curl -s localhost:32500/api/v1/health && echo'
if [ -n "${SUDO_PASSWORD:-}" ]; then
  # The remote shell reads the password from stdin and hands it to sudo -S
  # with a builtin printf: it never shows in ps or in a log.
  # shellcheck disable=SC2029 # $install and $after expand here, on purpose
  printf '%s\n' "$SUDO_PASSWORD" | ssh "$target" \
    "IFS= read -r pw && printf '%s\\n' \"\$pw\" | sudo -S -p '' sh -c '$install' && $after"
else
  ssh -t "$target" "sudo sh -c '$install' && $after"
fi
