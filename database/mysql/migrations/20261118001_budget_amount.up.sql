ALTER TABLE budgets ADD COLUMN amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (amount_cents >= 0);
