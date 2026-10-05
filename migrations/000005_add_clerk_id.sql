-- Link local users to Clerk identities. Existing rows keep a NULL clerk_id
-- until the Clerk webhook backfills them; new rows are created by webhooks.
ALTER TABLE users ADD COLUMN IF NOT EXISTS clerk_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS users_clerk_id_key ON users (clerk_id);
