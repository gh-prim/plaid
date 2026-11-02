-- Step 21 (admin V2, Invitations): the admin can copy a pending link again,
-- so its token is kept until the invitation is used, cancelled or expired.
-- Only admins read it; the hash stays the lookup key.
ALTER TABLE invitations ADD COLUMN token TEXT;
