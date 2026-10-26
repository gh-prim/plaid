# PLAID roadmap

Index of all features. Update it whenever an artifact's status changes.

**Legend:** — = not written yet · 📝 draft · ✅ accepted / approved · 🚧 in progress · ✔️ done

## Decisions (ADRs)
| ID | Decision | Status |
|----|----------|--------|
| [0001](decisions/0001-server-language.md) | Server language: Go | 📝 proposed |
| [0002](decisions/0002-monorepo-and-api-contract.md) | Monorepo + OpenAPI contract | 📝 proposed |
| [0003](decisions/0003-remote-access-v1.md) | Remote access v1: pure self-host | 📝 proposed |

## Features

| ID | Feature | Phase | Apps | Owner | Intent | Spec | Plan |
|----|---------|-------|------|-------|--------|------|------|
| [F001](features/F001-monorepo-foundations/intent.md) | Monorepo foundations, CI, Docker | 0 | all | ? | 📝 | — | — |
| [F002](features/F002-design-system/intent.md) | Design system and art direction | 0 | web, apple, android | ? | 📝 | — | — |
| [F003](features/F003-accounts-auth/intent.md) | Accounts and authentication | 1 | server, web | ? | 📝 | — | — |
| [F004](features/F004-library-scanning/intent.md) | Libraries and file scanning | 1 | server | ? | 📝 | — | — |
| [F005](features/F005-metadata/intent.md) | Metadata and artwork | 1 | server | ? | 📝 | — | — |
| [F006](features/F006-playback-streaming/intent.md) | Playback: direct play and transcoding | 1 | server | ? | 📝 | — | — |
| [F007](features/F007-web-onboarding-admin/intent.md) | Web: onboarding and administration | 2 | web | ? | 📝 | — | — |
| [F008](features/F008-web-browsing-player/intent.md) | Web: browsing and player | 2 | web | ? | 📝 | — | — |
| [F009](features/F009-library-sharing/intent.md) | Library sharing | 3 | server, web | ? | 📝 | — | — |
| [F010](features/F010-plaid-party/intent.md) | Plaid Party (watch together) | 3 | server, web | ? | 📝 | — | — |

### Later (intents to write)
- Apple app (iOS, iPadOS, tvOS): phase 4
- Android + Android TV app: phase 5
- Offline downloads, Plaid Hub, music, photos, Live TV: phase 6
- Intro and credits detection, trickplay

## Possible parallel work for two
Once F001 is done:
- **Dev A**: F004 → F005 (libraries and metadata)
- **Dev B**: F003 → F006 (accounts and playback)
- F002 (art direction) can move forward in parallel with everything else.
