-- F010: the organizer can turn off the 15-minute reminder of a scheduled
-- party (VoteModal board). The lobby still opens 15 minutes before.
ALTER TABLE parties ADD COLUMN no_reminder INTEGER NOT NULL DEFAULT 0;
