-- F005: matched metadata, people, collections, artwork, manual
-- identifications, sidecar files and server-wide settings.
-- Ids are UUIDv7 TEXT, timestamps RFC 3339 UTC. F004's items table is left
-- as the scanner writes it; matched data lives next to it.

ALTER TABLE libraries ADD COLUMN metadata_language TEXT NOT NULL DEFAULT 'en';  -- ISO 639-1
ALTER TABLE libraries ADD COLUMN metadata_region   TEXT NOT NULL DEFAULT 'US';  -- ISO 3166-1 alpha-2

CREATE TABLE collections (
  id          TEXT PRIMARY KEY,
  library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  tmdb_id     INTEGER NOT NULL,
  name        TEXT NOT NULL,
  overview    TEXT,
  updated_at  TEXT NOT NULL,
  UNIQUE (library_id, tmdb_id)
);

CREATE TABLE artwork (
  id           TEXT PRIMARY KEY,
  owner_type   TEXT NOT NULL CHECK (owner_type IN ('item','person','collection')),
  owner_id     TEXT NOT NULL,              -- cleaned up with its owner by the store
  kind         TEXT NOT NULL CHECK (kind IN ('poster','backdrop','logo','clearart','banner','thumb','still','portrait')),
  source       TEXT NOT NULL CHECK (source IN ('local','tmdb','fanart')),
  remote_url   TEXT,                       -- allow-listed hosts only
  local_path   TEXT,                       -- read in place (R13)
  language     TEXT,
  width        INTEGER,
  height       INTEGER,
  votes        REAL,
  sha256       TEXT,                       -- NULL until downloaded (lazy, R17)
  ext          TEXT,
  colors       TEXT,                       -- JSON (R21)
  selected     INTEGER NOT NULL DEFAULT 0,
  locked       INTEGER NOT NULL DEFAULT 0, -- R30
  local_mtime  TEXT,
  fetched_at   TEXT,
  created_at   TEXT NOT NULL
);
CREATE INDEX artwork_owner ON artwork(owner_type, owner_id, kind);
CREATE INDEX artwork_sha ON artwork(sha256);

CREATE TABLE item_metadata (
  item_id          TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
  state            TEXT NOT NULL DEFAULT 'pending'
                   CHECK (state IN ('pending','matched','needs_review','unmatched')),
  match_source     TEXT CHECK (match_source IN ('nfo','hint','auto','manual','parent')),
  confidence       REAL,
  match_locked     INTEGER NOT NULL DEFAULT 0,
  tmdb_id          INTEGER,                  -- seasons and episodes: the show's id
  imdb_id          TEXT,
  tvdb_id          INTEGER,
  title            TEXT,
  original_title   TEXT,
  sort_title       TEXT,
  overview         TEXT,
  tagline          TEXT,
  release_date     TEXT,                     -- movie release, show first air, episode air date
  end_date         TEXT,                     -- show last air date
  show_status      TEXT,
  runtime_minutes  INTEGER,
  rating           REAL,
  rating_count     INTEGER,
  certification    TEXT,
  certification_region TEXT,
  age_rating       INTEGER,                  -- R24: 0, 7, 10, 12, 13, 16, 18 or NULL (unrated); seasons and episodes copy their show's
  genres           TEXT,                     -- JSON [{"id": 878, "name": "Science-Fiction"}]
  studios          TEXT,                     -- JSON [string]
  trailers         TEXT,                     -- JSON (R22)
  collection_id    TEXT REFERENCES collections(id) ON DELETE SET NULL,
  duplicate_of     TEXT REFERENCES items(id) ON DELETE SET NULL,
  field_languages  TEXT,                     -- JSON {"overview": "en"} when fallen back
  locked_fields    TEXT,                     -- JSON ["title","poster"]
  local_fields     TEXT,                     -- JSON: fields taken from .nfo
  attempts         INTEGER NOT NULL DEFAULT 0,
  next_attempt_at  TEXT,                     -- retries (R33)
  refresh_at       TEXT,                     -- scheduled refresh (R31)
  last_error       TEXT,
  fetched_at       TEXT,
  updated_at       TEXT NOT NULL
);
CREATE INDEX item_metadata_queue ON item_metadata(state, next_attempt_at);
CREATE INDEX item_metadata_refresh ON item_metadata(refresh_at);
CREATE INDEX item_metadata_tmdb ON item_metadata(tmdb_id);

CREATE TABLE match_candidates (
  item_id        TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  rank           INTEGER NOT NULL,
  tmdb_id        INTEGER NOT NULL,
  title          TEXT NOT NULL,
  original_title TEXT,
  year           INTEGER,
  overview       TEXT,
  poster_path    TEXT,                       -- TMDB path, served through the candidate proxy
  confidence     REAL NOT NULL,
  PRIMARY KEY (item_id, rank)
);

CREATE TABLE people (
  id            TEXT PRIMARY KEY,
  tmdb_id       INTEGER UNIQUE,
  name          TEXT NOT NULL,
  portrait_id   TEXT REFERENCES artwork(id) ON DELETE SET NULL,
  updated_at    TEXT NOT NULL
);

CREATE TABLE item_credits (
  item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  person_id  TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
  role       TEXT NOT NULL CHECK (role IN ('cast','director','writer','creator')),
  character  TEXT,
  ord        INTEGER NOT NULL,
  PRIMARY KEY (item_id, person_id, role)
);
CREATE INDEX item_credits_person ON item_credits(person_id);

CREATE TABLE file_identifications (
  file_id         TEXT PRIMARY KEY REFERENCES media_files(id) ON DELETE CASCADE,
  kind            TEXT NOT NULL CHECK (kind IN ('movie','episode')),
  tmdb_id         INTEGER NOT NULL,          -- movie id, or show id for episodes
  season_number   INTEGER,
  episode_numbers TEXT,                      -- JSON [int]
  created_by      TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at      TEXT NOT NULL
);

CREATE TABLE sidecar_files (
  path        TEXT PRIMARY KEY,
  library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  size        INTEGER NOT NULL,
  mtime       TEXT NOT NULL,
  seen_at     TEXT NOT NULL
);
CREATE INDEX sidecar_files_library ON sidecar_files(library_id);

CREATE TABLE settings (
  key         TEXT PRIMARY KEY,
  value       TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
