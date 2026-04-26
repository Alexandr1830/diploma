-- =============================================================================
-- Migration: 006_rule_sets.sql
-- Description: Configurable compliance rules for GOST/internal standards.
-- Admin defines named rule sets; each set contains typed rules; running a
-- set against a document version persists a compliance_checks record with
-- per-rule pass/fail JSON.
-- =============================================================================

CREATE TYPE rule_type AS ENUM (
    'must_contain_phrase',
    'must_not_contain_phrase',
    'section_order',
    'regex_match',
    'min_word_count'
);

CREATE TYPE rule_severity AS ENUM ('error', 'warning');

-- ---------------------------------------------------------------------------
-- rule_sets — verzeichnis наборов правил.
-- ---------------------------------------------------------------------------
CREATE TABLE rule_sets (
    id           BIGSERIAL PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    is_active    BOOLEAN      NOT NULL DEFAULT TRUE,
    created_by   BIGINT       NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX rule_sets_name_uniq ON rule_sets (LOWER(name));

-- ---------------------------------------------------------------------------
-- rules — отдельные правила набора.
-- params — JSONB, форма зависит от rule_type:
--   must_contain_phrase     {"phrase":"Введение","case_sensitive":false}
--   must_not_contain_phrase {"phrase":"и т.д.","case_sensitive":false}
--   section_order           {"sections":["Введение","Глава 1","Заключение"],"case_sensitive":false}
--   regex_match             {"pattern":"^\\d+\\.","expect":"match"|"nomatch","flags":"i"}
--   min_word_count          {"min":1000}
-- ---------------------------------------------------------------------------
CREATE TABLE rules (
    id           BIGSERIAL PRIMARY KEY,
    rule_set_id  BIGINT        NOT NULL REFERENCES rule_sets(id) ON DELETE CASCADE,
    name         VARCHAR(255)  NOT NULL,
    rule_type    rule_type     NOT NULL,
    params       JSONB         NOT NULL DEFAULT '{}'::jsonb,
    severity     rule_severity NOT NULL DEFAULT 'error',
    position     INT           NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
CREATE INDEX rules_rule_set_id_idx ON rules (rule_set_id, position);

-- ---------------------------------------------------------------------------
-- compliance_checks — история запусков. results хранит массив
-- {rule_id, name, severity, passed, message, location}.
-- ---------------------------------------------------------------------------
CREATE TABLE compliance_checks (
    id            BIGSERIAL PRIMARY KEY,
    version_id    BIGINT      NOT NULL REFERENCES document_versions(id) ON DELETE CASCADE,
    rule_set_id   BIGINT      NOT NULL REFERENCES rule_sets(id) ON DELETE CASCADE,
    total_rules   INT         NOT NULL DEFAULT 0,
    passed_rules  INT         NOT NULL DEFAULT 0,
    failed_rules  INT         NOT NULL DEFAULT 0,
    results       JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_by    BIGINT      NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX compliance_checks_version_id_idx ON compliance_checks (version_id, created_at DESC);
