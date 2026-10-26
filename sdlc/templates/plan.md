---
id: FNNN
title: Short title
owner: name
reviewer: name
status: draft            # draft | approved | in-progress | done
spec: ./spec.md
branch: feat/FNNN-slug
---

# FNNN — Plan: Title

> This plan must be precise enough for the other dev to execute it alone.
> If the implementation diverges, this file is updated **in the same commit**.

## Files touched
| File | Action | Why |
|------|--------|-----|
| `server/internal/…` | create | … |

## Steps, in order
1. [ ] … (link to requirement `FNNN-Rx`)
2. [ ] …

## Risks
| Risk | Likelihood | Mitigation |
|------|------------|------------|
| What can break? | … | … |

**Riskiest step:** …

## Rejected options
- Option B: why we don't choose it.

## Expected evidence
What proves it's done, to paste into the PR:
- [ ] Tests added: …
- [ ] `make test` green
- [ ] Screenshots (UI) compared to the mockup
- [ ] Manual check: …

## Log
Deviations from the plan found during the build (date + what + why).
