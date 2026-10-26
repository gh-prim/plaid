# PR review — PLAID

Rules applied to every PR, by Claude and by the human reviewer alike.

## The passes
1. **Compliance**: the PR does what the feature's `spec.md` and `plan.md` say, no more, no less. If it deviates, `plan.md` has been updated.
2. **Bugs**: logic, edge cases, concurrency (streams, Party WebSockets), resource leaks (FFmpeg processes, files).
3. **Security and privacy**: access control on shared libraries, guests, filesystem path traversal, secrets, tokens.
4. **API contract**: every change is reflected in `api/openapi.yaml`, without breaking existing clients.
5. **Evidence**: tests present and relevant, `make test` green, screenshots for UI.

## Severity
- **Important**: reserved for behavior defects, data leaks and spec violations. Blocks the merge.
- **Should fix**: a real but non-blocking issue.
- **Nit**: 3 max per PR. Beyond that, hold back.

## Ignore
- Generated code (API clients, generated migrations).
- What CI already checks (format, lint).

## Approval
The author never approves their own PR: the other dev always approves.
