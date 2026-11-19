ALTER TABLE budgets ADD COLUMN amount_cents INTEGER NOT NULL DEFAULT 0 CHECK (amount_cents >= 0);
