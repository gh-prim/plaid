# PLAID SDLC

How we build PLAID, adapted from [Anthropic's AI-native SDLC playbook](https://claude.com/fr/resources/articles/the-ai-native-sdlc-playbook) for a team of two devs.

**Golden rule: no code without an accepted `plan.md`.**
Each step produces a versioned Markdown file that the next step reads. The Git history of these files is the audit trail: who decided what, when and why.

---

## The flow

```
 idea ──► intent.md ──► spec.md ──► plan.md ──► code + tests ──► PR ──► merge
           (what/        (how it      (files,       (checked by     (cross
            why)          works)       order,        Claude before   review)
                                       risks)        the PR)
   ▲                                                                   │
   └────────────── bug / feedback / idea in prod = new intent ◄────────┘
```

| Step | Artifact | Written by | Validated by | Moves to the next step when… |
|------|----------|------------|--------------|------------------------------|
| 1. Plan | `intent.md` | The idea's author + Claude (framing) | The other dev | Status `accepted`, merged |
| 2. Design | `spec.md` | Claude, from the intent | The other dev | Watch-outs resolved, status `approved` |
| 3. Build | `plan.md` then the code | Claude in Plan mode, challenged by the owner dev | The owner dev | The plan is precise enough for the other dev to execute it alone |
| 4. Test | tests + evidence in the PR | Claude | CI | `make test` green, screenshots attached for UI |
| 5. Deploy | PR | Claude | **The other dev** (never the author) | Approved per [`REVIEW.md`](../REVIEW.md) |
| 6. Maintain | new `intent.md` | Anyone | — | The loop starts again |

### Working as two, concretely
- Each feature has **one owner** (the `owner` field in the frontmatter). They drive the feature end to end.
- **The other dev reviews every artifact** (intent, spec, plan, PR). Nobody validates their own work.
- We work in parallel on features **that don't touch the same files**, each in our own worktree / branch.
- If the code drifts from the plan, we **update `plan.md` in the same commit**. If the spec has to change after the plan, we note it in the spec's "Revisions" section: it's a sign the spec was too thin.

---

## Layout

```
sdlc/
├── README.md            # this file: the process
├── vision.md            # product vision, apps, roadmap, art direction
├── roadmap.md           # index of all features and their status
├── decisions/           # ADRs: architecture decisions, one per file
│   └── NNNN-title.md
├── features/            # one feature = one folder
│   └── FNNN-slug/
│       ├── intent.md    # step 1
│       ├── spec.md      # step 2
│       └── plan.md      # step 3
└── templates/           # templates to copy (or let the skills do it)
```

Files at the repo root complete the setup:
- [`CLAUDE.md`](../CLAUDE.md): context Claude reads every session (commands, conventions, common mistakes).
- [`REVIEW.md`](../REVIEW.md): PR review rules.
- `.claude/skills/`: the skills that automate each step (`new-intent`, `intent-to-spec`, `spec-to-plan`).

---

## Statuses

In each artifact's frontmatter:

| Artifact | Possible statuses |
|----------|-------------------|
| `intent.md` | `draft` → `accepted` / `rejected` |
| `spec.md` | `draft` → `approved` |
| `plan.md` | `draft` → `approved` → `in-progress` → `done` |
| ADR | `proposed` → `accepted` / `superseded` |

The overall feature status is kept up to date in [`roadmap.md`](roadmap.md).

## Conventions
- Features: `FNNN-kebab-case-slug` (e.g. `F010-plaid-party`). Numbers are never reused.
- ADRs: `NNNN-title.md` (e.g. `0001-server-language.md`).
- Branches: `feat/F010-plaid-party`, `fix/F006-seek-hls`.
- Commits and PRs cite the feature ID: `F006: basic HLS transcoding`.
- **Everything in the repo is written in English**: code, comments, commits, branches, PRs, API, logs, and also docs, specs, ADRs and skills. The team may chat in French.
- **All UI goes through i18n**: no hardcoded strings. Source locale `en`, first translation `fr`. Details in [`CLAUDE.md`](../CLAUDE.md).

## Using the skills
- "New idea: …" → `new-intent` skill: Claude asks framing questions, then creates `features/FNNN-slug/intent.md`.
- "Move F004 to spec" → `intent-to-spec` skill, only if the intent is `accepted`.
- "Plan F004" → `spec-to-plan` skill, only if the spec is `approved`. Do it in Plan mode.
