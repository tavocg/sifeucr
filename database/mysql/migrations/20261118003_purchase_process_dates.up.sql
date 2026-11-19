ALTER TABLE purchase_processes ADD COLUMN starts_at BIGINT, ADD COLUMN ends_at BIGINT;
UPDATE purchase_processes p JOIN budgets b ON b.id = p.budget_id
SET p.starts_at = b.starts_at, p.ends_at = b.ends_at;
ALTER TABLE purchase_processes MODIFY starts_at BIGINT NOT NULL, MODIFY ends_at BIGINT NOT NULL,
  ADD CHECK (ends_at > starts_at);
