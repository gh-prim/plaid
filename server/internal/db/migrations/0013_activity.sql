-- Step 21 (admin V2): one row per playback that ended, for "hours watched
-- per day" and Activity. Names are copied: the log outlives accounts.
CREATE TABLE play_log (
  id            TEXT PRIMARY KEY,
  user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
  display_name  TEXT NOT NULL,
  profile_name  TEXT NOT NULL,
  item_id       TEXT NOT NULL,
  title         TEXT NOT NULL,
  device_name   TEXT NOT NULL,
  method        TEXT NOT NULL CHECK (method IN ('direct_play','direct_stream','transcode')),
  remote        INTEGER NOT NULL DEFAULT 0,
  started_at    TEXT NOT NULL,
  ended_at      TEXT NOT NULL,
  watched_ms    INTEGER NOT NULL
);
CREATE INDEX play_log_started ON play_log(started_at);
