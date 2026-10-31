-- Intro, recap, credits and preview of movies and episodes, as TheIntroDB
-- gave them, kept so a title is looked up once in a while, not per play.
CREATE TABLE item_segments (
  item_id    TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
  segments   TEXT NOT NULL,             -- JSON: [{"kind":"intro","startMs":0,"endMs":52000}]
  fetched_at TEXT NOT NULL
);
