---
id: 0004
title: License
status: accepted
date: 2026-10-26
deciders: [gh-prim]
supersedes: null
---

# 0004 — License

## Context
The repo is public on GitHub. Without a license, the code is visible but nobody may legally use, modify or redistribute it. The F001 spec required a decision before going public and before publishing any package or image.

## Options
### A. MIT
- ✅ As open as it gets while keeping attribution: anyone can use, modify, redistribute, including in closed products.
- ✅ Short, universally understood, compatible with every dependency we use.
- ❌ A company can fork PLAID into a closed product without giving changes back.

### B. Apache-2.0
- ✅ Same openness, plus an explicit patent grant.
- ❌ Longer, NOTICE-file obligations; little practical gain for this project.

### C. AGPL-3.0 (common for self-hosted projects)
- ✅ Forces anyone running a modified PLAID as a service to publish their changes.
- ❌ Not "super open": scares away contributors and integrators.

## Decision
We choose **MIT**: the request was a very open license, and MIT is the simplest, most permissive mainstream choice.

## Consequences
- `LICENSE` at the repo root, `license: MIT` in the `.deb` metadata and `web/package.json`.
- The F001 watch-out on the license is resolved; publishing release packages and the Docker image (R18) is no longer blocked by licensing.
- Contributions are accepted under the same license (inbound = outbound), no CLA.
