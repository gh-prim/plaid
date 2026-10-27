#!/usr/bin/env bash
# Prints the next release tag (vMAJOR.MINOR.PATCH) from the latest v* tag.
#   scripts/next-version.sh [patch|minor|major]
# Without an argument, the bump comes from the HEAD commit message:
# "#major" or "#minor" anywhere in it, otherwise patch. First release: v0.1.0.
set -euo pipefail

bump="${1:-}"
if [ -z "$bump" ]; then
  msg="$(git log -1 --format=%B)"
  case "$msg" in
    *"#major"*) bump="major" ;;
    *"#minor"*) bump="minor" ;;
    *) bump="patch" ;;
  esac
fi

last="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1)"
if [ -z "$last" ]; then
  echo v0.1.0
  exit 0
fi

IFS=. read -r major minor patch <<<"${last#v}"
case "$bump" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  *) echo "unknown bump: $bump (patch, minor or major)" >&2; exit 2 ;;
esac
echo "v$major.$minor.$patch"
