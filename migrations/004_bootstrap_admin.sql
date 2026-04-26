-- =============================================================================
-- Migration: 004_bootstrap_admin.sql
-- Description: Bootstrap data so the system has an entry point:
--              - Single admin user (admin@example.com / admin12345)
--              - One default project (id=1) and one category (id=1) referenced
--                by the document-creation form on the frontend.
--
-- ВАЖНО: Не использовать в production. Замените пароль admin'а сразу после первого входа.
-- =============================================================================

INSERT INTO users (name, email, password_hash, role, is_active, must_change_password, created_at, updated_at)
VALUES (
    'Bootstrap Admin',
    'admin@example.com',
    '$2b$10$JdHgaqvbxTZAnlEqOI310uOKmsD8UeApLbWXjfdTeR8.lcF1xgCEW',  -- bcrypt('admin12345')
    'admin',
    TRUE,
    TRUE,
    NOW(),
    NOW()
);

-- Default project + category for the create-document form (hardcoded to id=1 in UI).
INSERT INTO projects (id, name, description, created_at)
VALUES (1, 'General', 'Default project for newly created documents', NOW());

INSERT INTO categories (id, name, description, created_at)
VALUES (1, 'General', 'Default category for newly created documents', NOW());

-- Bump sequences so future inserts pick the next free id.
SELECT setval('projects_id_seq',   GREATEST((SELECT MAX(id) FROM projects),   1));
SELECT setval('categories_id_seq', GREATEST((SELECT MAX(id) FROM categories), 1));
