# PLAID WebSocket events

Real-time events exchanged between the server and clients over WebSocket.
REST endpoints live in [`openapi.yaml`](openapi.yaml); this file documents what OpenAPI can't.

_Empty for now. The Plaid Party protocol (F010) will be specified here: connection, room lifecycle, playback sync, presence, reactions, chat._

## Conventions (apply to every future event)
- Messages are JSON objects: `{ "type": "namespace.event", "data": { … }, "ts": <server unix ms> }`.
- `type` is a stable machine key, like `error.code` in the REST API.
- Server time (`ts`) is the reference clock; clients never trust their own clock for sync.
