ALTER TABLE taxpayers ADD COLUMN reta_base_cents INTEGER NOT NULL DEFAULT 0;
ALTER TABLE taxpayers ADD COLUMN reta_base_from TEXT NOT NULL DEFAULT '';
ALTER TABLE taxpayers ADD COLUMN reta_net_income_cents INTEGER NOT NULL DEFAULT 0;
ALTER TABLE taxpayers ADD COLUMN reta_net_income_year INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS reta_base_history (
    id                 TEXT PRIMARY KEY,
    taxpayer_id        TEXT NOT NULL REFERENCES taxpayers(id) ON DELETE CASCADE,
    base_cents         INTEGER NOT NULL,
    base_from          TEXT NOT NULL,
    net_income_cents   INTEGER NOT NULL,
    net_income_year    INTEGER NOT NULL,
    created_at         TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_reta_base_history_taxpayer
    ON reta_base_history (taxpayer_id, base_from DESC, created_at DESC);
