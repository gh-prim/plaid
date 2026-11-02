-- Admin monitoring (map, bandwidth, device health): what each play sent
-- and from where. A remote play keeps a city and rough coordinates from
-- the geolocation database, never the address itself.
ALTER TABLE play_log ADD COLUMN bitrate_bps INTEGER;   -- what the server sent, on average
ALTER TABLE play_log ADD COLUMN reasons TEXT;          -- why it wasn't played as is, comma-separated
ALTER TABLE play_log ADD COLUMN country TEXT;          -- ISO 3166-1 alpha-2
ALTER TABLE play_log ADD COLUMN city TEXT;
ALTER TABLE play_log ADD COLUMN lat REAL;
ALTER TABLE play_log ADD COLUMN lon REAL;
