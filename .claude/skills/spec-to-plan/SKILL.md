---
name: spec-to-plan
description: Produces the implementation plan.md for a PLAID feature from its approved spec. Use when asked to "plan FNNN", "make the plan for FNNN" / "planifier FNNN", "faire le plan de FNNN", or before starting to code a feature.
---

# Write a plan.md

1. Open `sdlc/features/FNNN-*/spec.md`. **If its status is not `approved`, stop** and say so.
2. Explore the relevant existing code before planning. Don't assume the structure: check it.
3. Copy `sdlc/templates/plan.md` into the feature folder and fill it in:
   - Every file created or modified, with the reason.
   - Small, ordered steps, each linked to an `FNNN-Rx` requirement. Tests come before or with the code, never after.
   - The risks, with **the riskiest step** named.
   - The rejected options and why.
   - The evidence expected in the PR (tests, screenshots, manual checks).
4. Write the plan.md in English, but keep talking to the user in their own language.
5. **Challenge your own plan** before handing it over: what can break? What touches files the other dev is working on (see the `in-progress` plans in `sdlc/features/`)? Could the other dev execute it alone?
6. Status `draft`. Update `sdlc/roadmap.md`.
7. No code is written until the owner has moved the plan to `approved`.
