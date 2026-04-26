-- Migration: 003_add_has_images.sql
-- Add has_images flag to document_versions for image detection during PDF parsing.

ALTER TABLE document_versions ADD COLUMN IF NOT EXISTS has_images BOOLEAN NOT NULL DEFAULT FALSE;
