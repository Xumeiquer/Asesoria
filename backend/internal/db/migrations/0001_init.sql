CREATE TABLE IF NOT EXISTS settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS taxpayers (
    id                TEXT PRIMARY KEY,
    nif               TEXT NOT NULL,
    name              TEXT NOT NULL,
    legal_form        TEXT NOT NULL CHECK (legal_form IN ('autonomo', 'sl')),
    email             TEXT NOT NULL DEFAULT '',
    phone             TEXT NOT NULL DEFAULT '',
    vat_regime        TEXT NOT NULL DEFAULT 'general',
    irpf_method       TEXT NOT NULL DEFAULT 'directa',
    has_employees     INTEGER NOT NULL DEFAULT 0,
    has_rentals       INTEGER NOT NULL DEFAULT 0,
    economic_activity INTEGER NOT NULL DEFAULT 1,
    cnae              TEXT NOT NULL DEFAULT '',
    fiscal_year_end   TEXT NOT NULL DEFAULT '12-31',
    iban              TEXT NOT NULL DEFAULT '',
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
    id               TEXT PRIMARY KEY,
    taxpayer_id      TEXT REFERENCES taxpayers(id) ON DELETE SET NULL,
    filing_id        TEXT,
    kind             TEXT NOT NULL DEFAULT 'other',
    filename         TEXT NOT NULL,
    mime             TEXT NOT NULL,
    size             INTEGER NOT NULL,
    sha256           TEXT NOT NULL,
    storage_path     TEXT NOT NULL,
    doc_date         TEXT NOT NULL DEFAULT '',
    amount_cents     INTEGER,
    counterparty_nif TEXT NOT NULL DEFAULT '',
    metadata_json    TEXT NOT NULL DEFAULT '{}',
    created_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS filings (
    id                  TEXT PRIMARY KEY,
    taxpayer_id         TEXT NOT NULL REFERENCES taxpayers(id) ON DELETE CASCADE,
    model_code          TEXT NOT NULL,
    model_name          TEXT NOT NULL,
    entity              TEXT NOT NULL,
    period_label        TEXT NOT NULL,
    period_start        TEXT NOT NULL,
    period_end          TEXT NOT NULL,
    due_date            TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'in_progress', 'ready', 'presented', 'late', 'not_applicable')),
    data_json           TEXT NOT NULL DEFAULT '{}',
    computed_json       TEXT NOT NULL DEFAULT '{}',
    notes               TEXT NOT NULL DEFAULT '',
    presented_at        TEXT,
    receipt_document_id TEXT REFERENCES documents(id) ON DELETE SET NULL,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    UNIQUE (taxpayer_id, model_code, period_label)
);

CREATE TABLE IF NOT EXISTS invoices (
    id                TEXT PRIMARY KEY,
    taxpayer_id       TEXT NOT NULL REFERENCES taxpayers(id) ON DELETE CASCADE,
    direction         TEXT NOT NULL CHECK (direction IN ('issued', 'received')),
    number            TEXT NOT NULL,
    invoice_date      TEXT NOT NULL,
    counterparty_nif  TEXT NOT NULL,
    counterparty_name TEXT NOT NULL,
    concept           TEXT NOT NULL DEFAULT '',
    base_cents        INTEGER NOT NULL,
    vat_rate          INTEGER NOT NULL,
    vat_cents         INTEGER NOT NULL,
    retention_rate    INTEGER NOT NULL DEFAULT 0,
    retention_cents   INTEGER NOT NULL DEFAULT 0,
    total_cents       INTEGER NOT NULL,
    category          TEXT NOT NULL DEFAULT '',
    document_id       TEXT REFERENCES documents(id) ON DELETE SET NULL,
    created_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pairing_codes (
    code       TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at    TEXT
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TEXT NOT NULL,
    last_used_at TEXT
);

CREATE TABLE IF NOT EXISTS audit_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    at           TEXT NOT NULL,
    actor        TEXT NOT NULL,
    action       TEXT NOT NULL,
    entity       TEXT NOT NULL,
    entity_id    TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_filings_taxpayer_due ON filings (taxpayer_id, due_date);
CREATE INDEX IF NOT EXISTS idx_filings_status ON filings (status);
CREATE INDEX IF NOT EXISTS idx_documents_taxpayer ON documents (taxpayer_id, created_at);
CREATE INDEX IF NOT EXISTS idx_invoices_taxpayer_date ON invoices (taxpayer_id, invoice_date);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log (entity, entity_id);
