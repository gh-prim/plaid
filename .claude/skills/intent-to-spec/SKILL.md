---
name: intent-to-spec
description: Turns an accepted intent.md into a spec.md (requirements + design) for a PLAID feature. Use when asked to "move FNNN to spec", "spec FNNN" or "write the spec for …" / "passer FNNN en spec", "spécifier FNNN", "écrire la spec de …".
---

# Write a spec.md

1. Open `sdlc/features/FNNN-*/intent.md`. **If its status is not `accepted`, stop** and say so.
2. Read the context: `sdlc/vision.md` (art direction, architecture), the accepted ADRs in `sdlc/decisions/`, the specs of the features this one depends on, and `api/openapi.yaml` if it exists.
3. Copy `sdlc/templates/spec.md` into the feature folder and fill in every section:
   - Requirements numbered `FNNN-R1`, `FNNN-R2`… with a priority of `must`, `should` or `could`.
   - The data model and API are concrete (table names, routes, payloads) and consistent with what already exists.
   - UI copy follows the PLAID art direction tone (warm, "plaid & chill").
   - Acceptance criteria use the "Given … when … then …" format and are testable.
4. Write the spec.md in English, but keep talking to the user in their own language.
5. **Explicitly flag the watch-outs**: contradictions with an ADR or another spec, security or privacy topics (sharing, guests, remote access), performance on modest hardware, open questions from the intent still unanswered. Don't invent the answers: list them.
6. If a structural architecture decision comes up, propose an ADR in `sdlc/decisions/` (status `proposed`) instead of settling it in the spec.
7. Status `draft`. Update `sdlc/roadmap.md`.
8. Finish by listing the watch-outs the reviewer must settle before moving the spec to `approved`.
