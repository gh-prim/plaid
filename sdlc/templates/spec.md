---
id: FNNN
title: Short title
owner: name
reviewer: name
status: draft            # draft | approved
intent: ./intent.md
decisions: []            # linked ADRs, e.g. [0001, 0003]
---

# FNNN — Spec: Title

## Summary
What the feature does, in one paragraph.

## User journeys
Step-by-step scenarios, for each type of user affected.

1. …
2. …

## Functional requirements
| ID | Requirement | Priority |
|----|-------------|----------|
| FNNN-R1 | The system must… | must |
| FNNN-R2 | … | should |

## Design

### Data model
New or modified tables and fields, with their relationships.

### API
REST endpoints and WebSocket events, to be carried into `api/openapi.yaml`.

| Method | Route | Description |
|--------|-------|-------------|
| GET | `/api/v1/…` | … |

### Behavior per app
- **server**: …
- **web**: …
- **apple / android**: … (or "out of scope for now")

### UI / Art direction
Screens involved, states (empty, loading, error), copy tone. Link to a mockup if one exists.

### i18n
New translation keys (`namespace.key`) with the source `en` text and the `fr` translation. Plurals, dates and durations involved. Check that the UI holds up with longer text (German runs about 30% longer than English).

## Non-functional requirements
Performance (response time, CPU on a NAS), security, accessibility, i18n.

## ⚠️ Watch-outs
Risks, contradictions, security or privacy topics. Each one must be **resolved** or **tracked** before moving to `approved`.

- [ ] …

## Acceptance criteria
Written to be testable. These are what `plan.md` turns into tests.

- [ ] Given…, when…, then…

## Revisions
Changes made to the spec after the plan was written (date + reason).
