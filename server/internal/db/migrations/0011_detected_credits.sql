-- Where an episode's end credits start, as the server found them in the
-- file (black frames), when TheIntroDB doesn't know or is unsure.
ALTER TABLE item_segments ADD COLUMN credits_ms INTEGER;
ALTER TABLE item_segments ADD COLUMN detected_at TEXT;
