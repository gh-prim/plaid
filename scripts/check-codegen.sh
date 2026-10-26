#!/usr/bin/env bash
# Regenerates the API code into a temp dir and compares it with the committed
# files. Fails when api/openapi.yaml changed without `make generate`, or when
# generated files were edited by hand.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Same config as the real generation, only the output path differs.
sed "s#^output:.*#output: $tmp/api.gen.go#" "$root/server/internal/api/oapi-codegen.yaml" >"$tmp/oapi-codegen.yaml"
(cd "$root/server" && go tool oapi-codegen -config "$tmp/oapi-codegen.yaml" "$root/api/openapi.yaml")
(cd "$root/web" && pnpm -s exec openapi-typescript ../api/openapi.yaml -o "$tmp/schema.d.ts" >/dev/null && pnpm -s exec prettier --stdin-filepath src/api/schema.d.ts <"$tmp/schema.d.ts" >"$tmp/schema.formatted.d.ts")

status=0
diff -u "$root/server/internal/api/api.gen.go" "$tmp/api.gen.go" || status=1
diff -u "$root/web/src/api/schema.d.ts" "$tmp/schema.formatted.d.ts" || status=1

if [ "$status" -ne 0 ]; then
  echo "❌ Generated API code is out of date. Run 'make generate' and commit the result." >&2
  exit 1
fi
echo "✅ generated API code is up to date"
