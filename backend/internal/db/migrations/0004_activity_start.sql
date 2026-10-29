ALTER TABLE taxpayers ADD COLUMN activity_start TEXT NOT NULL DEFAULT '';

-- Existing records do not know the real start date. Use the first day of the
-- year they were registered so an upgrade never hides periods of that same
-- year. New records send the real fiscal start date from the taxpayer form.
UPDATE taxpayers
SET activity_start = substr(created_at, 1, 4) || '-01-01'
WHERE activity_start = '';

-- "late" was an editable status that duplicated the derived "overdue"
-- condition. Keep the data but normalise it to the open state.
UPDATE filings
SET status = 'pending',
    updated_at = datetime('now')
WHERE status = 'late';

CREATE INDEX IF NOT EXISTS idx_filings_due ON filings (due_date);
