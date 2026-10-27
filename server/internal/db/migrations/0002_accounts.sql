-- F003: accounts, profiles, sessions, TV pairing.
-- Ids are UUIDv7 TEXT, timestamps RFC 3339 UTC TEXT with millisecond precision.

CREATE TABLE users (
  id                    TEXT PRIMARY KEY,
  username              TEXT NOT NULL UNIQUE COLLATE NOCASE,
  display_name          TEXT NOT NULL,
  password_hash         TEXT NOT NULL,              -- argon2id encoded ($argon2id$v=19$m=...)
  role                  TEXT NOT NULL CHECK (role IN ('admin','member')),
  locale                TEXT NOT NULL DEFAULT 'en',
  must_change_password  INTEGER NOT NULL DEFAULT 0,
  disabled_at           TEXT,
  created_at            TEXT NOT NULL,
  updated_at            TEXT NOT NULL
);

CREATE TABLE profiles (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name            TEXT NOT NULL,
  avatar_color    INTEGER NOT NULL DEFAULT 0 CHECK (avatar_color BETWEEN 0 AND 7),
  pin_hash        TEXT,                              -- argon2id, NULL = no PIN
  max_age         INTEGER,                           -- NULL = no limit
  allow_unrated   INTEGER NOT NULL DEFAULT 1,
  locale          TEXT,                              -- NULL = account locale
  is_default      INTEGER NOT NULL DEFAULT 0,
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL
);
CREATE UNIQUE INDEX profiles_one_default ON profiles(user_id) WHERE is_default = 1;
CREATE INDEX profiles_user ON profiles(user_id);

CREATE TABLE sessions (
  id              TEXT PRIMARY KEY,
  user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  profile_id      TEXT REFERENCES profiles(id) ON DELETE SET NULL,
  token_hash      BLOB NOT NULL UNIQUE,              -- sha256(token)
  device_name     TEXT NOT NULL,
  device_type     TEXT NOT NULL CHECK (device_type IN ('web','ios','ipados','tvos','android','androidtv','other')),
  client_version  TEXT,
  last_ip         TEXT,
  created_at      TEXT NOT NULL,
  last_seen_at    TEXT NOT NULL,
  expires_at      TEXT NOT NULL,                     -- last_seen_at + 90 days
  revoked_at      TEXT
);
CREATE INDEX sessions_user ON sessions(user_id);

CREATE TABLE pairing_codes (
  id                 TEXT PRIMARY KEY,
  code               TEXT NOT NULL UNIQUE,           -- 6 chars, normalized uppercase, no dash
  poll_secret_hash   BLOB NOT NULL,
  device_name        TEXT NOT NULL,
  device_type        TEXT NOT NULL,
  client_version     TEXT,
  created_at         TEXT NOT NULL,
  expires_at         TEXT NOT NULL,
  claimed_at         TEXT,
  claimed_session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL,
  token_delivered_at TEXT                            -- the token is handed over once
);
