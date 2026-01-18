-- Phase0 identity: anonymous session (cookie) with optional future user association
CREATE TABLE IF NOT EXISTS sessions (
  id VARCHAR(255) PRIMARY KEY,
  user_id VARCHAR(255) NULL,
  created_at VARCHAR(255) NOT NULL,
  last_seen_at VARCHAR(255) NOT NULL
);

-- Phase0 scan records
CREATE TABLE IF NOT EXISTS scans (
  id VARCHAR(255) PRIMARY KEY,
  session_id VARCHAR(255) NOT NULL,
  user_id VARCHAR(255) NULL,
  source VARCHAR(255) NOT NULL,
  status VARCHAR(255) NOT NULL,
  created_at VARCHAR(255) NOT NULL,
  FOREIGN KEY (session_id) REFERENCES sessions(id)
);

CREATE TABLE IF NOT EXISTS scan_images (
  id VARCHAR(255) PRIMARY KEY,
  scan_id VARCHAR(255) NOT NULL UNIQUE,
  storage_path TEXT NOT NULL,
  mime_type VARCHAR(255) NOT NULL,
  sha256 VARCHAR(64) NULL,
  width INTEGER NULL,
  height INTEGER NULL,
  created_at VARCHAR(255) NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ocr_results (
  id VARCHAR(255) PRIMARY KEY,
  scan_id VARCHAR(255) NOT NULL UNIQUE,
  model VARCHAR(255) NOT NULL,
  language VARCHAR(10) NULL,
  raw_text TEXT NOT NULL,
  structured_json TEXT NULL,
  prompt_version VARCHAR(50) NOT NULL,
  created_at VARCHAR(255) NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);

-- Phase0 annotations (generated from highlight selections)
CREATE TABLE IF NOT EXISTS annotations (
  id VARCHAR(255) PRIMARY KEY,
  scan_id VARCHAR(255) NOT NULL,
  ocr_result_id VARCHAR(255) NOT NULL,
  selected_text TEXT NOT NULL,
  selection_start INTEGER NULL,
  selection_end INTEGER NULL,
  meaning TEXT NOT NULL,
  usage_example TEXT NOT NULL,
  when_to_use TEXT NOT NULL,
  word_breakdown TEXT NOT NULL,
  alternative_meanings TEXT NOT NULL,
  model VARCHAR(255) NOT NULL,
  prompt_version VARCHAR(50) NOT NULL,
  created_at VARCHAR(255) NOT NULL,
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE,
  FOREIGN KEY (ocr_result_id) REFERENCES ocr_results(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scans_session_id_created_at ON scans(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_annotations_scan_id_created_at ON annotations(scan_id, created_at);
