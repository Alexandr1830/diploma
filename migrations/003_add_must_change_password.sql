-- =============================================================================
-- Migration: 004_must_change_password.sql
-- Description: Add must_change_password flag to users.
--              Existing users default to FALSE (no forced change).
--              New users created by admin will be set to TRUE explicitly.
-- =============================================================================

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;
