-- Migration 001: Drop old tables (if they exist from Phase 0)
-- This cleanup migration removes the old session-based schema

DROP TABLE IF EXISTS annotations CASCADE;
DROP TABLE IF EXISTS ocr_results CASCADE;
DROP TABLE IF EXISTS scan_images CASCADE;
DROP TABLE IF EXISTS scans CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS users CASCADE;
