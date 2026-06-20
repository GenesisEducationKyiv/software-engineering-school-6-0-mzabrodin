-- Confirmation tokens are now stateless JWTs; the column (and its UNIQUE constraint) is no longer needed.
ALTER TABLE subscriptions DROP COLUMN IF EXISTS confirm_token;

-- Release tracking moved to the scanner's watched_repos table.
ALTER TABLE repositories DROP COLUMN IF EXISTS last_seen_tag;
ALTER TABLE repositories DROP COLUMN IF EXISTS checked_at;
