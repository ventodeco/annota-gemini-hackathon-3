-- Migration 003: PRD reader memory, scan lifecycle, and source-aware annotations

ALTER TABLE users ALTER COLUMN preferred_language SET DEFAULT 'EN';
UPDATE users SET preferred_language = 'EN' WHERE preferred_language = 'ID';

ALTER TABLE scans ADD COLUMN IF NOT EXISTS source_type TEXT NOT NULL DEFAULT 'image';
ALTER TABLE scans ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'processing';
ALTER TABLE scans ADD COLUMN IF NOT EXISTS failure_reason TEXT;

UPDATE scans
SET
    source_type = CASE WHEN document_id IS NULL THEN 'image' ELSE 'pdf' END,
    status = CASE
        WHEN full_ocr_text IS NOT NULL AND LENGTH(TRIM(full_ocr_text)) > 0 THEN 'ready'
        ELSE status
    END;

ALTER TABLE documents ADD COLUMN IF NOT EXISTS last_page_number INTEGER NOT NULL DEFAULT 1;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS last_opened_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE documents ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE annotations ADD COLUMN IF NOT EXISTS source_type TEXT;
ALTER TABLE annotations ADD COLUMN IF NOT EXISTS document_id BIGINT;
ALTER TABLE annotations ADD COLUMN IF NOT EXISTS page_number INTEGER;
ALTER TABLE annotations ADD COLUMN IF NOT EXISTS source_label TEXT;

UPDATE annotations a
SET
    source_type = CASE WHEN s.document_id IS NULL THEN 'image' ELSE 'pdf' END,
    document_id = s.document_id,
    page_number = s.page_number,
    source_label = CASE
        WHEN s.document_id IS NULL THEN 'OCR Scan #' || s.id::TEXT
        ELSE 'PDF Document #' || s.document_id::TEXT || ', page ' || s.page_number::TEXT
    END
FROM scans s
WHERE a.scan_id = s.id
  AND a.source_type IS NULL;

CREATE INDEX IF NOT EXISTS idx_scans_user_status ON scans(user_id, status);
CREATE INDEX IF NOT EXISTS idx_documents_user_updated ON documents(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_annotations_document_page ON annotations(user_id, document_id, page_number);
