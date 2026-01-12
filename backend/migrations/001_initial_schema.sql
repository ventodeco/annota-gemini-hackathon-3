PRAGMA foreign_keys = ON;

-- Phase0 identity: anonymous session (cookie) with optional future user association
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NULL,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL
);

-- Phase0 scan records
CREATE TABLE IF NOT EXISTS scans (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL,
  user_id TEXT NULL,
  source TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE IF NOT EXISTS scan_images (
  id TEXT PRIMARY KEY,
  scan_id TEXT NOT NULL UNIQUE,
  storage_path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  sha256 TEXT NULL,
  width INTEGER NULL,
  height INTEGER NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ocr_results (
  id TEXT PRIMARY KEY,
  scan_id TEXT NOT NULL UNIQUE,
  model TEXT NOT NULL,
  language TEXT NULL,
  raw_text TEXT NOT NULL,
  structured_json TEXT NULL,
  prompt_version TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

-- Phase0 annotations (generated from highlight selections)
CREATE TABLE IF NOT EXISTS annotations (
  id TEXT PRIMARY KEY,
  scan_id TEXT NOT NULL,
  ocr_result_id TEXT NOT NULL,
  selected_text TEXT NOT NULL,
  selection_start INTEGER NULL,
  selection_end INTEGER NULL,
  meaning TEXT NOT NULL,
  usage_example TEXT NOT NULL,
  when_to_use TEXT NOT NULL,
  word_breakdown TEXT NOT NULL,
  alternative_meanings TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt_version TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE,
  FOREIGN KEY (ocr_result_id) REFERENCES ocr_results(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scans_session_id_created_at ON scans(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_annotations_scan_id_created_at ON annotations(scan_id, created_at);
