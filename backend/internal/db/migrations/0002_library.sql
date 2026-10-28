ALTER TABLE documents ADD COLUMN text_content TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN ocr_status TEXT NOT NULL DEFAULT 'none';
ALTER TABLE documents ADD COLUMN review_status TEXT NOT NULL DEFAULT 'ok';
ALTER TABLE documents ADD COLUMN source_path TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN library_path TEXT NOT NULL DEFAULT '';
ALTER TABLE documents ADD COLUMN storage_root TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS library_files (
    path        TEXT PRIMARY KEY,
    taxpayer_id TEXT,
    mtime_ns    INTEGER NOT NULL,
    size        INTEGER NOT NULL,
    sha256      TEXT NOT NULL DEFAULT '',
    document_id TEXT,
    status      TEXT NOT NULL DEFAULT 'imported',
    error       TEXT NOT NULL DEFAULT '',
    scanned_at  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_library_files_status ON library_files (status, scanned_at);
CREATE INDEX IF NOT EXISTS idx_documents_review ON documents (review_status, created_at);
CREATE INDEX IF NOT EXISTS idx_documents_date ON documents (doc_date);
