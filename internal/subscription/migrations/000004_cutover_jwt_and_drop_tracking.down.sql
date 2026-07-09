ALTER TABLE repositories ADD COLUMN IF NOT EXISTS checked_at TIMESTAMPTZ;
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS last_seen_tag TEXT;

-- Restored as nullable without the original UNIQUE constraint: existing rows
-- have no token to repopulate, so a NOT NULL UNIQUE column cannot be rebuilt.
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS confirm_token TEXT;