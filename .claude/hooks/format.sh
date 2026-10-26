#!/usr/bin/env bash
# PostToolUse hook: format the file Claude just edited. Fast, scoped to one
# file, never blocks (always exits 0). Heavy checks belong to `make test`.
set -uo pipefail

file="$(jq -r '.tool_input.file_path // empty')"
[ -n "$file" ] && [ -f "$file" ] || exit 0

root="$(cd "$(dirname "$0")/../.." && pwd)"

case "$file" in
  *.go)
    gofmt -w "$file" 2>/dev/null
    ;;
  "$root"/web/*.ts | "$root"/web/*.tsx | "$root"/web/*.js | "$root"/web/*.json | "$root"/web/*.css | "$root"/web/*.html)
    (cd "$root/web" && pnpm -s exec prettier --write --log-level silent "$file") 2>/dev/null
    ;;
esac
exit 0
