---
id: F002
title: Design system and art direction
owner: TBD
reviewer: TBD
status: draft
phase: 0
apps: [web, apple, android]
created: 2026-10-26
---

# F002 — Design system and art direction

## Problem
PLAID needs a strong identity (the plaid, the couch, "Netflix & chill") and has to stay consistent across five very different surfaces: web, iPhone, iPad, Apple TV and Android TV. Without a design system, each app will end up with its own interpretation.

## Expected outcome
A single design system (tokens, typography, iconography, components) designed in Claude Design, then exported as tokens that all three codebases consume (CSS variables, Swift, Kotlin).

## Users and systems affected
- The web, Apple and Android app devs.
- End users, who navigate by touch (mobile), mouse (web) and remote (TV, with a strong "focus" state).

## Success criteria
- [ ] Tokens: colors (dark by default + light), type, spacing, radii, shadows, motion.
- [ ] Key components designed with all their states (see [claude-design-prompt.md](claude-design-prompt.md)).
- [ ] Reference screens: home, movie detail, player, Plaid Party room.
- [ ] 10-foot (TV) variant validated for focus navigation.
- [ ] Tokens exported to `design/tokens.json`.

## Constraints
- Readable on a TV from 3 m away and on a phone in full sunlight.
- WCAG AA contrast at minimum.
- The tartan pattern stays subtle: it must never get in the way of reading posters.

## Out of scope
The final logo and the marketing site.

## Open questions
- [ ] Font: an open-licensed font (for self-hosting) or system fonts?
- [ ] A single signature tartan, or one tartan per user profile?
