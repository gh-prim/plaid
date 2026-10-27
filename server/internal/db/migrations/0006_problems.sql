-- F007: things that need the admin's attention, for the admin home and
-- the Problems screen (R29). One row per problem: the same code and params
-- bump count and last_seen instead of adding a row.

CREATE TABLE problems (
  id           TEXT PRIMARY KEY,           -- UUIDv7
  code         TEXT NOT NULL,              -- hw_fallback, ffmpeg_restarted, folder_unavailable, folder_unreadable, watch_limit, provider_key_refused
  params       TEXT NOT NULL,              -- JSON object of strings; never tokens or keys
  dedupe_key   TEXT NOT NULL UNIQUE,       -- code + canonical params
  first_seen   TEXT NOT NULL,
  last_seen    TEXT NOT NULL,
  count        INTEGER NOT NULL DEFAULT 1,
  resolved_at  TEXT
);
CREATE INDEX problems_last_seen ON problems(last_seen);
