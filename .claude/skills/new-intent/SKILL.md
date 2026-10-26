---
name: new-intent
description: Frames a new PLAID feature idea and creates its intent.md. Use when someone proposes an idea, a feature, a need or a bug report to turn into work ("new idea", "we could", "we should", "create an intent" / "nouvelle idée", "on pourrait faire", "il faudrait que", "crée un intent").
---

# Create an intent.md

1. Read `sdlc/vision.md` and `sdlc/roadmap.md` to place the idea and check it doesn't already exist. If a similar feature exists, suggest enriching it instead.
2. Act as the analyst and ask the missing questions, **all at once** and no more than 5:
   - What problem, for whom (admin, member, invited friend, guest via link)?
   - What observable outcome once it's done?
   - Which apps are affected (server, web, apple, android)?
   - Constraints (modest hardware, privacy, dependencies)?
   - What is explicitly out of scope?
3. Take the next free number `FNNN` (the highest ID in `sdlc/features/` + 1, never reused).
4. Copy `sdlc/templates/intent.md` to `sdlc/features/FNNN-slug/intent.md` and fill it in **using the author's words**. Describe the what and the why, never the technical how.
5. Write the intent.md in English (the slug too), but keep talking to the user in their own language.
6. Status `draft`. The owner is the author of the idea, the reviewer is the other dev.
7. Add the feature's row to `sdlc/roadmap.md`.
8. Finish by reminding that the other dev must review the intent and set its status to `accepted` before any spec.
