-- Migration 005: Map Clerk identities to the existing numeric users.id.
-- Additive only: users.id and every ownership foreign key stay unchanged.

ALTER TABLE users ADD COLUMN IF NOT EXISTS clerk_user_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_clerk_user_id
    ON users(clerk_user_id)
    WHERE clerk_user_id IS NOT NULL;

-- Permanent record of Clerk subjects whose ANNOTA account was deleted, so a
-- still-valid session token cannot recreate the profile.
CREATE TABLE IF NOT EXISTS deleted_clerk_users (
    clerk_user_id TEXT PRIMARY KEY,
    deleted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
