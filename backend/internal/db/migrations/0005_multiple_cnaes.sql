ALTER TABLE taxpayers ADD COLUMN cnaes_json TEXT NOT NULL DEFAULT '[]';

-- Preserve the single CNAE stored by previous versions as the first item of
-- the new list. New records keep the legacy cnae column pointing at the first
-- code for compatibility with older exports.
UPDATE taxpayers
SET cnaes_json = '["' || trim(cnae) || '"]'
WHERE trim(cnae) <> '';
