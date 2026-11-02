-- F009: sharing groups, per-person exceptions, invitations, profile avatars.
-- Ids are UUIDv7 TEXT, timestamps RFC 3339 UTC TEXT with millisecond precision.

-- A household-only library is seen by admins and household groups only.
ALTER TABLE libraries ADD COLUMN household_only INTEGER NOT NULL DEFAULT 0;

CREATE TABLE groups (
  id                   TEXT PRIMARY KEY,
  name                 TEXT NOT NULL UNIQUE COLLATE NOCASE,
  household            INTEGER NOT NULL DEFAULT 0,  -- every library, present and future
  max_quality          TEXT NOT NULL DEFAULT 'original'
                       CHECK (max_quality IN ('original','2160p','1080p','720p','480p')),
  max_streams          INTEGER CHECK (max_streams BETWEEN 1 AND 20),  -- NULL = no limit
  downloads            INTEGER NOT NULL DEFAULT 0,
  party                TEXT NOT NULL DEFAULT 'join' CHECK (party IN ('none','join','start')),
  can_create_profiles  INTEGER NOT NULL DEFAULT 0,
  created_at           TEXT NOT NULL,
  updated_at           TEXT NOT NULL
);

CREATE TABLE group_libraries (
  group_id    TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  PRIMARY KEY (group_id, library_id)
);
CREATE INDEX group_libraries_library ON group_libraries(library_id);

CREATE TABLE group_members (
  group_id  TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user ON group_members(user_id);

-- Exceptions: rights given to one person on top of their groups. They only
-- add; NULL takes the groups' value.
CREATE TABLE user_rights (
  user_id              TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  max_quality          TEXT CHECK (max_quality IN ('original','2160p','1080p','720p','480p')),
  max_streams          INTEGER CHECK (max_streams BETWEEN 1 AND 20),
  unlimited_streams    INTEGER,
  downloads            INTEGER,
  party                TEXT CHECK (party IN ('none','join','start')),
  can_create_profiles  INTEGER
);

CREATE TABLE user_libraries (
  user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  library_id  TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, library_id)
);
CREATE INDEX user_libraries_library ON user_libraries(library_id);

CREATE TABLE invitations (
  id          TEXT PRIMARY KEY,
  token_hash  BLOB NOT NULL UNIQUE,             -- sha256(token)
  name        TEXT NOT NULL,
  created_by  TEXT REFERENCES users(id) ON DELETE SET NULL,
  created_at  TEXT NOT NULL,
  expires_at  TEXT NOT NULL,
  used_at     TEXT,
  used_by     TEXT REFERENCES users(id) ON DELETE SET NULL,
  revoked_at  TEXT
);

CREATE TABLE invitation_groups (
  invitation_id  TEXT NOT NULL REFERENCES invitations(id) ON DELETE CASCADE,
  group_id       TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  PRIMARY KEY (invitation_id, group_id)
);

-- Avatars: the initial, a built avatar (options as JSON) or a photo.
ALTER TABLE profiles ADD COLUMN avatar_kind TEXT NOT NULL DEFAULT 'initial'
  CHECK (avatar_kind IN ('initial','builder','photo'));
ALTER TABLE profiles ADD COLUMN avatar_options TEXT;
ALTER TABLE profiles ADD COLUMN avatar_version INTEGER NOT NULL DEFAULT 0;

CREATE TABLE profile_photos (
  profile_id  TEXT PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
  jpeg        BLOB NOT NULL                      -- 512 px square, no metadata
);

-- Members from before F009 saw every library: they go into Family, a
-- household group with every right.
INSERT INTO groups (id, name, household, max_quality, max_streams, downloads, party, can_create_profiles, created_at, updated_at)
  SELECT '01900000-0000-7000-8000-000000000001', 'Family', 1, 'original', NULL, 1, 'start', 1,
         strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), strftime('%Y-%m-%dT%H:%M:%fZ', 'now');
INSERT INTO group_members (group_id, user_id)
  SELECT '01900000-0000-7000-8000-000000000001', id FROM users WHERE role = 'member';
