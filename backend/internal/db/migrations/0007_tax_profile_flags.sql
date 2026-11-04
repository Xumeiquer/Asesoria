ALTER TABLE taxpayers ADD COLUMN has_intracommunity_operations INTEGER NOT NULL DEFAULT 0;
ALTER TABLE taxpayers ADD COLUMN withholds_professional_income INTEGER NOT NULL DEFAULT 0;
ALTER TABLE taxpayers ADD COLUMN exempt_from_irpf_fractional_payment INTEGER NOT NULL DEFAULT 0;
ALTER TABLE taxpayers ADD COLUMN uses_sii INTEGER NOT NULL DEFAULT 0;
