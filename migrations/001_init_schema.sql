-- =============================================================================
-- Migration: 001_init_schema.sql
-- Description: Initial schema for the Document Management System
-- Tables: users, projects, categories, documents, document_versions,
--         review_comments, review_actions, discussion_threads,
--         discussion_messages, ai_checks, audit_logs, system_errors
-- =============================================================================

-- ---------------------------------------------------------------------------
-- Extensions
-- ---------------------------------------------------------------------------
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- ENUM types
-- ---------------------------------------------------------------------------
CREATE TYPE user_role AS ENUM ('writer', 'reviewer', 'developer', 'admin');

CREATE TYPE document_status AS ENUM (
    'draft',
    'in_review',
    'needs_revision',
    'approved',
    'published',
    'archived'
);

CREATE TYPE file_type AS ENUM ('docx', 'pdf', 'txt', 'md', 'yaml');

CREATE TYPE check_type AS ENUM ('AI', 'GOST');

CREATE TYPE check_status AS ENUM ('ok', 'warning', 'error');

CREATE TYPE thread_type AS ENUM ('general', 'anchored');

CREATE TYPE thread_status AS ENUM ('open', 'resolved');

CREATE TYPE review_action AS ENUM ('approve', 'request_revision');

-- ---------------------------------------------------------------------------
-- users
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT          NOT NULL,
    email         TEXT          NOT NULL UNIQUE,
    password_hash TEXT          NOT NULL,
    role          user_role     NOT NULL DEFAULT 'writer',
    is_active     BOOLEAN       NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_role  ON users (role);

-- ---------------------------------------------------------------------------
-- projects
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS projects (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- categories
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS categories (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ---------------------------------------------------------------------------
-- documents
-- Note: current_version_id and published_version_id use deferred FKs
--       to avoid a circular dependency at insert time.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS documents (
    id                   BIGSERIAL        PRIMARY KEY,
    title                TEXT             NOT NULL,
    description          TEXT             NOT NULL DEFAULT '',
    project_id           BIGINT           NOT NULL REFERENCES projects(id),
    category_id          BIGINT           NOT NULL REFERENCES categories(id),
    created_by           BIGINT           NOT NULL REFERENCES users(id),
    reviewer_id          BIGINT                    REFERENCES users(id),
    status               document_status  NOT NULL DEFAULT 'draft',
    current_version_id   BIGINT,
    published_version_id BIGINT,
    published_at         TIMESTAMPTZ,
    published_by         BIGINT                    REFERENCES users(id),
    created_at           TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documents_status      ON documents (status);
CREATE INDEX idx_documents_project_id  ON documents (project_id);
CREATE INDEX idx_documents_created_by  ON documents (created_by);
CREATE INDEX idx_documents_reviewer_id ON documents (reviewer_id);

-- ---------------------------------------------------------------------------
-- document_versions
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS document_versions (
    id             BIGSERIAL   PRIMARY KEY,
    document_id    BIGINT      NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version_number TEXT        NOT NULL,
    file_name      TEXT        NOT NULL,
    file_path      TEXT        NOT NULL,
    file_type      file_type   NOT NULL,
    parsed_text    TEXT,
    change_summary TEXT,
    uploaded_by    BIGINT      NOT NULL REFERENCES users(id),
    is_current     BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dv_document_id ON document_versions (document_id);
CREATE INDEX idx_dv_is_current  ON document_versions (document_id, is_current);

-- Back-fill FK constraints on documents now that document_versions exists
ALTER TABLE documents
    ADD CONSTRAINT fk_documents_current_version
        FOREIGN KEY (current_version_id)   REFERENCES document_versions(id) DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE documents
    ADD CONSTRAINT fk_documents_published_version
        FOREIGN KEY (published_version_id) REFERENCES document_versions(id) DEFERRABLE INITIALLY DEFERRED;

-- ---------------------------------------------------------------------------
-- review_comments  (writer ↔ reviewer internal review)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS review_comments (
    id           BIGSERIAL   PRIMARY KEY,
    document_id  BIGINT      NOT NULL REFERENCES documents(id)        ON DELETE CASCADE,
    version_id   BIGINT      NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    author_id    BIGINT      NOT NULL REFERENCES users(id),
    comment_text TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rc_version_id  ON review_comments (version_id);
CREATE INDEX idx_rc_document_id ON review_comments (document_id);

-- ---------------------------------------------------------------------------
-- review_actions  (approve / request_revision history)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS review_actions (
    id          BIGSERIAL     PRIMARY KEY,
    document_id BIGINT        NOT NULL REFERENCES documents(id)        ON DELETE CASCADE,
    version_id  BIGINT        NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    reviewer_id BIGINT        NOT NULL REFERENCES users(id),
    action      review_action NOT NULL,
    note        TEXT          NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ra_document_id ON review_actions (document_id);

-- ---------------------------------------------------------------------------
-- discussion_threads  (developer ↔ writer/reviewer, anchored to a version)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS discussion_threads (
    id          BIGSERIAL     PRIMARY KEY,
    document_id BIGINT        NOT NULL REFERENCES documents(id)        ON DELETE CASCADE,
    version_id  BIGINT        NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    created_by  BIGINT        NOT NULL REFERENCES users(id),
    thread_type thread_type   NOT NULL DEFAULT 'general',
    page_number INT,
    anchor_text TEXT,
    status      thread_status NOT NULL DEFAULT 'open',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_dt_version_id  ON discussion_threads (version_id);
CREATE INDEX idx_dt_document_id ON discussion_threads (document_id);
CREATE INDEX idx_dt_status      ON discussion_threads (status);

-- ---------------------------------------------------------------------------
-- discussion_messages
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS discussion_messages (
    id           BIGSERIAL   PRIMARY KEY,
    thread_id    BIGINT      NOT NULL REFERENCES discussion_threads(id) ON DELETE CASCADE,
    author_id    BIGINT      NOT NULL REFERENCES users(id),
    message_text TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dm_thread_id ON discussion_messages (thread_id);

-- ---------------------------------------------------------------------------
-- ai_checks  (AI and GOST validation results)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS ai_checks (
    id          BIGSERIAL    PRIMARY KEY,
    document_id BIGINT       NOT NULL REFERENCES documents(id)        ON DELETE CASCADE,
    version_id  BIGINT       NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    check_type  check_type   NOT NULL,
    score       NUMERIC(5,2),
    status      check_status NOT NULL,
    result_json TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ac_version_id  ON ai_checks (version_id);
CREATE INDEX idx_ac_document_id ON ai_checks (document_id);
CREATE INDEX idx_ac_check_type  ON ai_checks (check_type);

-- ---------------------------------------------------------------------------
-- audit_logs
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
    id          BIGSERIAL   PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id),
    action      TEXT        NOT NULL,
    entity_type TEXT        NOT NULL,
    entity_id   BIGINT      NOT NULL,
    details     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_al_user_id    ON audit_logs (user_id);
CREATE INDEX idx_al_entity     ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_al_created_at ON audit_logs (created_at DESC);

-- ---------------------------------------------------------------------------
-- system_errors
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS system_errors (
    id            BIGSERIAL   PRIMARY KEY,
    service_name  TEXT        NOT NULL,
    error_message TEXT        NOT NULL,
    error_context TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_se_service_name ON system_errors (service_name);
CREATE INDEX idx_se_created_at   ON system_errors (created_at DESC);
