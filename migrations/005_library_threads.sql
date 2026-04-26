-- =============================================================================
-- Migration: 005_library_threads.sql
-- Description: Adds is_public to discussion_threads to distinguish:
--                false (default) — internal review thread (writer ↔ reviewer ↔ admin)
--                true            — public library thread (visible to all authenticated users
--                                 on /library/:id once the document is published)
-- =============================================================================

ALTER TABLE discussion_threads
    ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_dt_is_public ON discussion_threads (document_id, is_public);
