-- Rotten Tomatoes scores from MDBList, out of 100: the Tomatometer
-- (critics) and the Popcornmeter (audience). NULL when unknown.
ALTER TABLE item_metadata ADD COLUMN critic_score INTEGER;
ALTER TABLE item_metadata ADD COLUMN audience_score INTEGER;
