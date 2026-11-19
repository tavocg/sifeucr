ALTER TABLE purchase_processes ADD COLUMN starts_at BIGINT, ADD COLUMN ends_at BIGINT;
UPDATE purchase_processes p SET starts_at = b.starts_at, ends_at = b.ends_at
FROM budgets b WHERE b.id = p.budget_id;
ALTER TABLE purchase_processes ALTER COLUMN starts_at SET NOT NULL, ALTER COLUMN ends_at SET NOT NULL,
  ADD CHECK (ends_at > starts_at);
