-- Issue #35: identify the configured producer (provider/gateway/region/model/
-- prompt version) of each OCR result. Additive and nullable: existing rows keep
-- their text and stay unknown (NULL); nothing is backfilled.
ALTER TABLE scans ADD COLUMN IF NOT EXISTS ocr_provenance JSONB;
