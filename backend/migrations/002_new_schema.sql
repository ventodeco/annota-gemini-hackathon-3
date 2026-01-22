-- Migration 002: Create new schema for Phase 1 (Authentication & User Data)
-- Users: Supports multiple OAuth providers (Google, Apple, GitHub)

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    preferred_language VARCHAR(10) DEFAULT 'ID',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (provider, provider_id)
);

-- Scans: User's scanned images with embedded OCR text

CREATE TABLE scans (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL,
    full_ocr_text TEXT,
    detected_language VARCHAR(10),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Annotations: User's saved highlights with AI-generated explanations

CREATE TABLE annotations (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    scan_id BIGINT REFERENCES scans(id) ON DELETE SET NULL,
    highlighted_text TEXT NOT NULL,
    context_text TEXT,
    nuance_data JSONB NOT NULL,
    is_bookmarked BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for fast retrieval

CREATE INDEX idx_scans_user_id ON scans(user_id);
CREATE INDEX idx_annotations_user_id ON annotations(user_id);
