-- F010: Plaid Party (rooms, invitations, guests, votes) and the Upcoming tab
-- (followed shows, upcoming episodes, release alerts). Live room state stays
-- in memory; these tables hold what must outlive a connection.
-- Ids are UUIDv7 TEXT, timestamps RFC 3339 UTC TEXT with millisecond precision.

CREATE TABLE parties (
  id                    TEXT PRIMARY KEY,
  code                  TEXT NOT NULL,              -- 6 chars, F003-R20 alphabet
  status                TEXT NOT NULL CHECK (status IN ('scheduled','voting','lobby','live','ended','cancelled')),
  -- The party's title; NULL while a vote is open. A party planned on an
  -- episode not on the server yet points at it by show + numbers (R38).
  item_id               TEXT REFERENCES items(id) ON DELETE SET NULL,
  future_show_id        TEXT REFERENCES items(id) ON DELETE CASCADE,
  future_season         INTEGER,
  future_episode        INTEGER,
  organizer_user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  organizer_profile_id  TEXT REFERENCES profiles(id) ON DELETE SET NULL,
  control_mode          TEXT NOT NULL DEFAULT 'host' CHECK (control_mode IN ('host','everyone')),
  scheduled_at          TEXT,
  note                  TEXT,
  created_at            TEXT NOT NULL,
  started_at            TEXT,
  ended_at              TEXT,
  summary_json          TEXT                         -- end summary (R21), kept 30 days
);
CREATE UNIQUE INDEX parties_live_code ON parties(code) WHERE status NOT IN ('ended','cancelled');
CREATE INDEX parties_status ON parties(status, scheduled_at);

CREATE TABLE party_invites (
  party_id     TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  answer       TEXT NOT NULL DEFAULT 'pending' CHECK (answer IN ('pending','yes','maybe','no')),
  invited_at   TEXT NOT NULL,
  answered_at  TEXT,
  reminded_at  TEXT,
  PRIMARY KEY (party_id, user_id)
);
CREATE INDEX party_invites_user ON party_invites(user_id);

CREATE TABLE party_guests (
  id            TEXT PRIMARY KEY,
  party_id      TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
  first_name    TEXT NOT NULL,
  avatar_color  INTEGER NOT NULL DEFAULT 0,
  token_hash    BLOB NOT NULL UNIQUE,               -- sha256(token)
  created_at    TEXT NOT NULL,
  removed_at    TEXT
);
CREATE INDEX party_guests_party ON party_guests(party_id);

-- People the host removed: they can't come back to that party (R16).
CREATE TABLE party_removed (
  party_id  TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
  user_id   TEXT REFERENCES users(id) ON DELETE CASCADE,
  guest_id  TEXT REFERENCES party_guests(id) ON DELETE CASCADE,
  CHECK ((user_id IS NULL) <> (guest_id IS NULL))
);
CREATE INDEX party_removed_party ON party_removed(party_id);

CREATE TABLE party_votes (
  party_id         TEXT PRIMARY KEY REFERENCES parties(id) ON DELETE CASCADE,
  proposals_until  TEXT NOT NULL,
  ranking_until    TEXT NOT NULL,
  seed             TEXT,                            -- tie draw, stored before drawing (R33)
  closed_at        TEXT,
  winner_item_id   TEXT REFERENCES items(id) ON DELETE SET NULL
);

CREATE TABLE party_proposals (
  party_id    TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
  item_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at  TEXT NOT NULL,
  PRIMARY KEY (party_id, item_id)
);

-- Ballots are read by the tally and by their own voter only (R32).
CREATE TABLE party_ballots (
  party_id     TEXT NOT NULL REFERENCES parties(id) ON DELETE CASCADE,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  first_item   TEXT REFERENCES items(id) ON DELETE SET NULL,
  second_item  TEXT REFERENCES items(id) ON DELETE SET NULL,
  third_item   TEXT REFERENCES items(id) ON DELETE SET NULL,
  veto_item    TEXT REFERENCES items(id) ON DELETE SET NULL,
  updated_at   TEXT NOT NULL,
  PRIMARY KEY (party_id, user_id)
);

CREATE TABLE show_follows (
  profile_id  TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  show_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  created_at  TEXT NOT NULL,
  PRIMARY KEY (profile_id, show_id)
);

CREATE TABLE upcoming_episodes (
  show_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  season      INTEGER NOT NULL,
  episode     INTEGER NOT NULL,
  tmdb_id     INTEGER,
  title       TEXT,
  air_date    TEXT NOT NULL,                        -- YYYY-MM-DD (TMDB gives no time)
  fetched_at  TEXT NOT NULL,
  PRIMARY KEY (show_id, season, episode)
);
CREATE INDEX upcoming_episodes_date ON upcoming_episodes(air_date);

CREATE TABLE release_alerts (
  profile_id   TEXT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  show_id      TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  season       INTEGER NOT NULL,
  episode      INTEGER NOT NULL,
  created_at   TEXT NOT NULL,
  notified_at  TEXT,
  PRIMARY KEY (profile_id, show_id, season, episode)
);
