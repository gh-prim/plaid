-- F006: watch state per profile and item, and the keyframe index used to
-- cut Direct Stream segments. Playback sessions live in memory only (R8);
-- playback settings use F005's settings table.

-- Watch state per profile and item, never per file (R33, F004-R14).
CREATE TABLE watch_state (
  profile_id      TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  item_id         TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  position_ms     INTEGER NOT NULL DEFAULT 0,
  duration_ms     INTEGER,
  watched         INTEGER NOT NULL DEFAULT 0,
  play_count      INTEGER NOT NULL DEFAULT 0,
  last_played_at  TEXT,
  last_file_id    TEXT REFERENCES media_files(id) ON DELETE SET NULL,
  updated_at      TEXT NOT NULL,
  PRIMARY KEY (profile_id, item_id)
);
CREATE INDEX watch_state_recent ON watch_state(profile_id, last_played_at);

-- Keyframe times from the container index (Matroska Cues, MP4 stss), R13.
CREATE TABLE media_keyframes (
  file_id     TEXT PRIMARY KEY REFERENCES media_files(id) ON DELETE CASCADE,
  file_mtime  TEXT NOT NULL,               -- invalidated when the file changes
  keyframes   BLOB NOT NULL,               -- delta-encoded uvarint, milliseconds
  created_at  TEXT NOT NULL
);
