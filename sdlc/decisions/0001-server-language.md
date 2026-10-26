---
id: 0001
title: Server language
status: accepted
date: 2026-10-26
deciders: [gh-prim]
supersedes: null
---

# 0001 — Server language

## Context
The server scans files, drives FFmpeg, serves HTTP/HLS streams and holds the Plaid Party WebSockets. It must run on modest hardware (NAS, Raspberry Pi) and be simple to install.

The code is written by AI, so the learning curve doesn't matter. What matters: real-world performance, the speed of the write → compile → test loop, ease of distribution, and how easily two humans can **review** the PRs.

**Where does CPU time actually go?** Into FFmpeg (C, called as a subprocess), the disk and the network. The server code mostly does I/O and orchestration. The language changes nothing about transcoding speed.

## Options
### A. Go
- ✅ Compiles in a few seconds: very short feedback loop for the agent.
- ✅ Static binary without CGO, amd64 / arm64 cross-compilation with one environment variable, ideal for NAS and Raspberry.
- ✅ Simple, uniform code, fast to review for two humans validating lots of generated PRs.
- ✅ Concurrency (goroutines) perfectly suited to hundreds of streams and WebSockets.
- ❌ GC: a few dozen MB of extra RAM, pauses in the microsecond range (imperceptible here).
- ❌ The type system lets more bugs through at compile time (nil, data races only detected at runtime with `-race`).

### B. Rust
- ✅ Maximum CPU and memory performance, no GC.
- ✅ The compiler catches many bugs (memory, data races, unhandled cases): a very good safety net for generated code.
- ❌ Much slower compilation (10 to 60 s incremental as the project grows): every agent iteration costs more.
- ❌ Async code (tokio, lifetimes, `Pin`) is more verbose and harder for humans to review.
- ❌ Cross-compilation gets painful as soon as there are C dependencies.

### C. TypeScript (Bun)
- ✅ Same language as the web.
- ❌ Less suited to a long-running daemon on small hardware.

## Decision
We choose **Go**. Rust's performance gain is real but applies to a part of the system that isn't the bottleneck (FFmpeg is). Go wins on what really matters with an agent: fast iterations, trivial distribution, easy-to-review PRs.

We offset Go's weaker typing with safeguards: `go test -race` in CI, strict `golangci-lint`, integration tests on the Party protocol.

## Consequences
- SQLite via a pure Go driver (`modernc.org/sqlite`) to keep a CGO-free binary.
- If a module becomes a real CPU hotspot (for example image analysis or trickplay thumbnail generation), we can isolate it in Rust later, behind a clear interface.
