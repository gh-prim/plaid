-- F008: what the catalog reads to sort, filter and search (spec data model).
-- Kept in sync by triggers, so the scanner (F004) and metadata (F005) write
-- paths don't change: item_genres from item_metadata.genres, a show's newest
-- episode date, and an FTS5 index of movies, shows, episodes and people.

ALTER TABLE items ADD COLUMN newest_child_added_at TEXT;  -- shows: created_at of their newest episode
CREATE INDEX items_library_added ON items(library_id, kind, created_at);

CREATE TABLE item_genres (
  item_id   TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  genre_id  INTEGER NOT NULL,                -- TMDB genre id, shared across languages
  name      TEXT NOT NULL,                   -- in the library's metadata language
  PRIMARY KEY (item_id, genre_id)
);
CREATE INDEX item_genres_genre ON item_genres(genre_id);

-- One row per indexed document, so its FTS rowid is stable and a trigger
-- deletes by rowid instead of scanning the index.
CREATE TABLE search_docs (
  rowid       INTEGER PRIMARY KEY,
  kind        TEXT NOT NULL CHECK (kind IN ('movie','show','episode','person')),
  ref_id      TEXT NOT NULL UNIQUE,          -- items.id or people.id
  library_id  TEXT                           -- NULL for people
);

CREATE VIRTUAL TABLE search_index USING fts5(
  title, original_title,
  kind UNINDEXED, ref_id UNINDEXED, library_id UNINDEXED,
  tokenize = 'unicode61 remove_diacritics 2',
  prefix = '1 2 3'
);

-- Items: the display title is the matched one, else the parsed one.
CREATE TRIGGER items_search_insert AFTER INSERT ON items
WHEN NEW.kind IN ('movie','show','episode')
BEGIN
  INSERT INTO search_docs (kind, ref_id, library_id) VALUES (NEW.kind, NEW.id, NEW.library_id);
  INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
    SELECT d.rowid, COALESCE(NULLIF(m.title, ''), NEW.title, ''), m.original_title, NEW.kind, NEW.id, NEW.library_id
    FROM search_docs d LEFT JOIN item_metadata m ON m.item_id = NEW.id
    WHERE d.ref_id = NEW.id;
END;

CREATE TRIGGER items_search_update AFTER UPDATE OF title ON items
WHEN NEW.kind IN ('movie','show','episode')
BEGIN
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = NEW.id);
  INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
    SELECT d.rowid, COALESCE(NULLIF(m.title, ''), NEW.title, ''), m.original_title, NEW.kind, NEW.id, NEW.library_id
    FROM search_docs d LEFT JOIN item_metadata m ON m.item_id = NEW.id
    WHERE d.ref_id = NEW.id;
END;

-- Also fires for the rows ON DELETE CASCADE removes (a show's episodes, a library's items).
CREATE TRIGGER items_search_delete AFTER DELETE ON items
WHEN OLD.kind IN ('movie','show','episode')
BEGIN
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = OLD.id);
  DELETE FROM search_docs WHERE ref_id = OLD.id;
END;

-- Episode → season → show; the date only moves forward.
CREATE TRIGGER items_newest_child_insert AFTER INSERT ON items
WHEN NEW.kind = 'episode'
BEGIN
  UPDATE items SET newest_child_added_at = NEW.created_at
  WHERE id = (SELECT parent_id FROM items WHERE id = NEW.parent_id)
    AND (newest_child_added_at IS NULL OR newest_child_added_at < NEW.created_at);
END;

CREATE TRIGGER items_newest_child_move AFTER UPDATE OF parent_id ON items
WHEN NEW.kind = 'episode'
BEGIN
  UPDATE items SET newest_child_added_at = NEW.created_at
  WHERE id = (SELECT parent_id FROM items WHERE id = NEW.parent_id)
    AND (newest_child_added_at IS NULL OR newest_child_added_at < NEW.created_at);
END;

-- Metadata: genres (id 0 comes from admin edits, which have no TMDB id) and the search title.
CREATE TRIGGER item_metadata_catalog_insert AFTER INSERT ON item_metadata
BEGIN
  INSERT OR IGNORE INTO item_genres (item_id, genre_id, name)
    SELECT NEW.item_id, json_extract(value, '$.id'), json_extract(value, '$.name')
    FROM json_each(CASE WHEN json_valid(NEW.genres) THEN NEW.genres ELSE '[]' END)
    WHERE json_extract(value, '$.id') > 0 AND json_extract(value, '$.name') IS NOT NULL;
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = NEW.item_id);
  INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
    SELECT d.rowid, COALESCE(NULLIF(NEW.title, ''), i.title, ''), NEW.original_title, d.kind, d.ref_id, d.library_id
    FROM search_docs d JOIN items i ON i.id = d.ref_id
    WHERE d.ref_id = NEW.item_id;
END;

CREATE TRIGGER item_metadata_catalog_update AFTER UPDATE OF title, original_title, genres ON item_metadata
BEGIN
  DELETE FROM item_genres WHERE item_id = NEW.item_id;
  INSERT OR IGNORE INTO item_genres (item_id, genre_id, name)
    SELECT NEW.item_id, json_extract(value, '$.id'), json_extract(value, '$.name')
    FROM json_each(CASE WHEN json_valid(NEW.genres) THEN NEW.genres ELSE '[]' END)
    WHERE json_extract(value, '$.id') > 0 AND json_extract(value, '$.name') IS NOT NULL;
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = NEW.item_id);
  INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
    SELECT d.rowid, COALESCE(NULLIF(NEW.title, ''), i.title, ''), NEW.original_title, d.kind, d.ref_id, d.library_id
    FROM search_docs d JOIN items i ON i.id = d.ref_id
    WHERE d.ref_id = NEW.item_id;
END;

CREATE TRIGGER item_metadata_catalog_delete AFTER DELETE ON item_metadata
BEGIN
  DELETE FROM item_genres WHERE item_id = OLD.item_id;
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = OLD.item_id);
  INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
    SELECT d.rowid, COALESCE(i.title, ''), NULL, d.kind, d.ref_id, d.library_id
    FROM search_docs d JOIN items i ON i.id = d.ref_id
    WHERE d.ref_id = OLD.item_id;
END;

-- People.
CREATE TRIGGER people_search_insert AFTER INSERT ON people
BEGIN
  INSERT INTO search_docs (kind, ref_id) VALUES ('person', NEW.id);
  INSERT INTO search_index (rowid, title, kind, ref_id)
    SELECT rowid, NEW.name, 'person', NEW.id FROM search_docs WHERE ref_id = NEW.id;
END;

CREATE TRIGGER people_search_update AFTER UPDATE OF name ON people
BEGIN
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = NEW.id);
  INSERT INTO search_index (rowid, title, kind, ref_id)
    SELECT rowid, NEW.name, 'person', NEW.id FROM search_docs WHERE ref_id = NEW.id;
END;

CREATE TRIGGER people_search_delete AFTER DELETE ON people
BEGIN
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = OLD.id);
  DELETE FROM search_docs WHERE ref_id = OLD.id;
END;

-- Backfill what existed before this migration.
UPDATE items SET newest_child_added_at = (
  SELECT MAX(e.created_at) FROM items s JOIN items e ON e.parent_id = s.id AND e.kind = 'episode'
  WHERE s.parent_id = items.id AND s.kind = 'season'
) WHERE kind = 'show';

INSERT OR IGNORE INTO item_genres (item_id, genre_id, name)
  SELECT m.item_id, json_extract(g.value, '$.id'), json_extract(g.value, '$.name')
  FROM item_metadata m, json_each(CASE WHEN json_valid(m.genres) THEN m.genres ELSE '[]' END) g
  WHERE json_extract(g.value, '$.id') > 0 AND json_extract(g.value, '$.name') IS NOT NULL;

INSERT INTO search_docs (kind, ref_id, library_id)
  SELECT kind, id, library_id FROM items WHERE kind IN ('movie','show','episode');
INSERT INTO search_docs (kind, ref_id) SELECT 'person', id FROM people;

INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
  SELECT d.rowid, COALESCE(NULLIF(m.title, ''), i.title, ''), m.original_title, d.kind, d.ref_id, d.library_id
  FROM search_docs d JOIN items i ON i.id = d.ref_id LEFT JOIN item_metadata m ON m.item_id = i.id;
INSERT INTO search_index (rowid, title, kind, ref_id)
  SELECT d.rowid, p.name, 'person', p.id FROM search_docs d JOIN people p ON p.id = d.ref_id;
