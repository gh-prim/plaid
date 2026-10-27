-- F004: libraries, folders, items, media files and their streams.
-- Ids are UUIDv7 TEXT, timestamps RFC 3339 UTC.

CREATE TABLE libraries (
  id               TEXT PRIMARY KEY,
  name             TEXT NOT NULL,
  type             TEXT NOT NULL CHECK (type IN ('movies','shows')),
  scan_interval_s  INTEGER,                 -- NULL = defaults (R19)
  last_scan_at     TEXT,
  created_at       TEXT NOT NULL,
  updated_at       TEXT NOT NULL
);

CREATE TABLE library_folders (
  id           TEXT PRIMARY KEY,
  library_id   TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  path         TEXT NOT NULL UNIQUE,        -- cleaned, symlinks resolved
  fs_type      TEXT,                        -- from statfs, e.g. ext4, nfs, cifs
  watched      INTEGER NOT NULL DEFAULT 0,
  status       TEXT NOT NULL DEFAULT 'ok' CHECK (status IN ('ok','unavailable','unreadable')),
  created_at   TEXT NOT NULL
);
CREATE INDEX library_folders_library ON library_folders(library_id);

CREATE TABLE items (
  id              TEXT PRIMARY KEY,
  library_id      TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  kind            TEXT NOT NULL CHECK (kind IN ('movie','show','season','episode')),
  parent_id       TEXT REFERENCES items(id) ON DELETE CASCADE,  -- season → show, episode → season
  title           TEXT,                     -- parsed; F005 adds matched metadata
  sort_title      TEXT,
  year            INTEGER,
  season_number   INTEGER,
  episode_number  INTEGER,
  air_date        TEXT,                     -- date-based episodes
  hints           TEXT,                     -- JSON: {"tmdb": "12345", "imdb": "tt…"}
  hidden          INTEGER NOT NULL DEFAULT 0, -- no file left (R17)
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE INDEX items_library_kind ON items(library_id, kind);
CREATE INDEX items_parent ON items(parent_id);

CREATE TABLE media_files (
  id              TEXT PRIMARY KEY,
  library_id      TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  folder_id       TEXT NOT NULL REFERENCES library_folders(id) ON DELETE CASCADE,
  path            TEXT NOT NULL UNIQUE,
  size            INTEGER NOT NULL,
  mtime           TEXT NOT NULL,
  fingerprint     BLOB,                     -- R16
  edition         TEXT,
  parse_status    TEXT NOT NULL CHECK (parse_status IN ('ok','unrecognized')),
  probe_status    TEXT NOT NULL DEFAULT 'pending' CHECK (probe_status IN ('pending','ok','failed','skipped')),
  container       TEXT,
  duration_ms     INTEGER,
  bitrate         INTEGER,
  width           INTEGER,
  height          INTEGER,
  video_codec     TEXT,
  hdr_format      TEXT,
  probe_json      TEXT,
  probed_at       TEXT,
  missing_since   TEXT,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE INDEX media_files_library ON media_files(library_id);
CREATE INDEX media_files_missing ON media_files(library_id, missing_since);
CREATE INDEX media_files_folder ON media_files(folder_id);

CREATE TABLE item_files (
  item_id  TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  file_id  TEXT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
  PRIMARY KEY (item_id, file_id)
);
CREATE INDEX item_files_file ON item_files(file_id);

CREATE TABLE media_streams (
  file_id     TEXT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
  idx         INTEGER NOT NULL,             -- ffprobe stream index; external subs use 1000+
  type        TEXT NOT NULL CHECK (type IN ('video','audio','subtitle')),
  codec       TEXT,
  language    TEXT,                         -- ISO 639-2/B, 'und' if unknown
  title       TEXT,
  channels    INTEGER,
  is_default  INTEGER NOT NULL DEFAULT 0,
  is_forced   INTEGER NOT NULL DEFAULT 0,
  is_sdh      INTEGER NOT NULL DEFAULT 0,
  is_image    INTEGER NOT NULL DEFAULT 0,   -- PGS, VobSub
  external_path TEXT,                       -- external subtitle file
  PRIMARY KEY (file_id, idx)
);
