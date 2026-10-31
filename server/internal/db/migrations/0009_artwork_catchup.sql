-- Artwork syncs that failed on a busy database (before 0008) were never
-- tried again: matched movies and shows with no poster, and shows whose
-- episodes have no still at all, are refreshed now. Since then a failed
-- sync brings the refresh forward by itself.
UPDATE item_metadata SET refresh_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE state = 'matched' AND item_id IN (
  SELECT i.id FROM items i
  WHERE i.kind IN ('movie', 'show') AND (
    NOT EXISTS (SELECT 1 FROM artwork a WHERE a.owner_type = 'item' AND a.owner_id = i.id AND a.kind = 'poster')
    OR (i.kind = 'show'
      AND EXISTS (SELECT 1 FROM items s JOIN items e ON e.parent_id = s.id WHERE s.parent_id = i.id AND e.kind = 'episode')
      AND NOT EXISTS (SELECT 1 FROM items s JOIN items e ON e.parent_id = s.id
        JOIN artwork a ON a.owner_type = 'item' AND a.owner_id = e.id AND a.kind = 'still'
        WHERE s.parent_id = i.id))
  )
);
