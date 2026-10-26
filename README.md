# PLAID

> Plaid & chill. Your cinema under the blanket.

PLAID is a self-hosted media server, an alternative to Plex: it indexes your movies and shows, streams them to every screen, and makes watching **together** a first-class feature.

**Status:** early development. Nothing to watch yet: grab a blanket and check back soon.

## Install
See [docs/install.md](docs/install.md): Docker, or a native package for Ubuntu/Debian.

## Develop
```bash
make setup   # install dependencies and tools
make dev     # server on :32500, web dev server on :5173
make test    # everything CI checks
```

How we work (intent → spec → plan → code): [sdlc/README.md](sdlc/README.md).
Project context for Claude Code: [CLAUDE.md](CLAUDE.md).
