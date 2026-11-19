CREATE TABLE purchase_processes_new (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  budget_id INTEGER NOT NULL REFERENCES budgets(id),
  starts_at INTEGER NOT NULL,
  ends_at INTEGER NOT NULL CHECK (ends_at > starts_at)
);
INSERT INTO purchase_processes_new (id, name, budget_id, starts_at, ends_at)
SELECT p.id, p.name, p.budget_id, b.starts_at, b.ends_at
FROM purchase_processes p JOIN budgets b ON b.id = p.budget_id;
DROP TABLE purchase_processes;
ALTER TABLE purchase_processes_new RENAME TO purchase_processes;
