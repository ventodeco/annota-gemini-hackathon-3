-- Ensure databases that applied migration 001 before the index was added receive it.
CREATE INDEX IF NOT EXISTS idx_annotations_scan_id ON annotations(scan_id);
