-- name: ClaimSetup :execrows
DELETE FROM pending_setup WHERE id = 1;

-- name: CreateBudget :exec
INSERT INTO budgets (name,item_number,starts_at,ends_at,amount_cents) VALUES (?,?,?,?,?);

-- name: CreateProcess :execrows
INSERT INTO purchase_processes (name,budget_id,starts_at,ends_at)
SELECT sqlc.arg(process_name), budgets.id, sqlc.arg(starts_at), sqlc.arg(ends_at) FROM budgets
WHERE budgets.id = sqlc.arg(budget_id)
  AND budgets.starts_at <= sqlc.arg(starts_at)
  AND budgets.ends_at >= sqlc.arg(ends_at);

-- name: ListBudgets :many
SELECT id,name,item_number,starts_at,ends_at,amount_cents FROM budgets ORDER BY starts_at DESC,id DESC;

-- name: ListProcesses :many
SELECT p.id,p.budget_id,p.name,b.name AS budget_name,p.starts_at,p.ends_at FROM purchase_processes p JOIN budgets b ON b.id=p.budget_id ORDER BY p.id DESC;
