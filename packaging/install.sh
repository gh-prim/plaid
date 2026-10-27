#!/usr/bin/env bash
# Installs PLAID on Ubuntu/Debian from the latest GitHub release:
#
#   curl -fsSL https://raw.githubusercontent.com/gh-prim/plaid/main/packaging/install.sh | sudo bash
#
# What it does: picks the .deb for your CPU, checks its SHA-256 against the
# release's checksums.txt, refreshes apt's package lists (the .deb depends on
# the distro's ffmpeg), then installs it with apt. Nothing else.
# Prefer doing it by hand? Download the .deb from the releases page and run
# `sudo apt install ./plaid_<version>_<arch>.deb`.
set -euo pipefail

repo="gh-prim/plaid"

fail() { echo "plaid installer: $*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || fail "please run as root (sudo)."
command -v apt-get >/dev/null || fail "only Ubuntu/Debian (apt) are supported."

case "$(uname -m)" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) fail "unsupported CPU architecture: $(uname -m)" ;;
esac

if [ -n "${PLAID_DEB_URL:-}" ]; then
  # Testing hook: install a specific package instead of the latest release.
  deb_url="$PLAID_DEB_URL"
  sums_url="${PLAID_CHECKSUMS_URL:?PLAID_CHECKSUMS_URL is required with PLAID_DEB_URL}"
else
  urls="$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
    grep -o '"browser_download_url": *"[^"]*"' | cut -d'"' -f4)"
  deb_url="$(grep "_${arch}\.deb\$" <<<"$urls" | head -n1 || true)"
  sums_url="$(grep '/checksums\.txt$' <<<"$urls" | head -n1 || true)"
  [ -n "$deb_url" ] || fail "no $arch package found in the latest release."
  [ -n "$sums_url" ] || fail "no checksums.txt found in the latest release."
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
deb="$(basename "$deb_url")"

echo "Downloading $deb…"
curl -fsSL -o "$tmp/$deb" "$deb_url"
curl -fsSL -o "$tmp/checksums.txt" "$sums_url"

echo "Verifying checksum…"
(cd "$tmp" && grep " ${deb}\$" checksums.txt | sha256sum -c --quiet -) ||
  fail "checksum mismatch for $deb, aborting."

echo "Installing…"
apt-get update -qq
apt-get install -y "$tmp/$deb"
echo "Done. Grab a blanket: PLAID is on port 32500."
