-- Migration 002: Add documents table and link scans to documents

CREATE TABLE IF NOT EXISTS documents (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    file_url TEXT NOT NULL,
    filename TEXT NOT NULL,
    page_count INTEGER NOT NULL,
    file_size BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_documents_user_id ON documents(user_id);

-- Make image_url nullable (PDF scans don't have images)
ALTER TABLE scans ALTER COLUMN image_url DROP NOT NULL;

-- Add document link columns to existing scans table
ALTER TABLE scans ADD COLUMN IF NOT EXISTS document_id BIGINT REFERENCES documents(id) ON DELETE SET NULL;
ALTER TABLE scans ADD COLUMN IF NOT EXISTS page_number INTEGER;
