-- F008: the search index is rewritten only when a name or title really
-- changes. "AFTER UPDATE OF name" fires on every UPDATE that sets the
-- column, and F005's upserts set it each time a movie's cast is written:
-- an FTS5 delete and insert per person, a second or more per movie on
-- modest hardware, holding the write lock meanwhile (SQLITE_BUSY for the
-- other writers).
DROP TRIGGER IF EXISTS people_search_update;
CREATE TRIGGER people_search_update AFTER UPDATE OF name ON people
WHEN OLD.name IS NOT NEW.name
BEGIN
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = NEW.id);
  INSERT INTO search_index (rowid, title, kind, ref_id)
    SELECT rowid, NEW.name, 'person', NEW.id FROM search_docs WHERE ref_id = NEW.id;
END;

DROP TRIGGER IF EXISTS item_metadata_catalog_update;
CREATE TRIGGER item_metadata_catalog_update AFTER UPDATE OF title, original_title, genres ON item_metadata
WHEN OLD.title IS NOT NEW.title OR OLD.original_title IS NOT NEW.original_title OR OLD.genres IS NOT NEW.genres
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

DROP TRIGGER IF EXISTS items_search_update;
CREATE TRIGGER items_search_update AFTER UPDATE OF title ON items
WHEN NEW.kind IN ('movie','show','episode') AND OLD.title IS NOT NEW.title
BEGIN
  DELETE FROM search_index WHERE rowid = (SELECT rowid FROM search_docs WHERE ref_id = NEW.id);
  INSERT INTO search_index (rowid, title, original_title, kind, ref_id, library_id)
    SELECT d.rowid, COALESCE(NULLIF(m.title, ''), NEW.title, ''), m.original_title, NEW.kind, NEW.id, NEW.library_id
    FROM search_docs d LEFT JOIN item_metadata m ON m.item_id = NEW.id
    WHERE d.ref_id = NEW.id;
END;
