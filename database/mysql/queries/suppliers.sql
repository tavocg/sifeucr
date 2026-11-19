-- name: CreateSupplier :execrows
INSERT INTO suppliers (identity_number, name, description, email, phone, iban, budget_id)
SELECT sqlc.arg(identity_number), sqlc.arg(supplier_name), sqlc.arg(description),
       sqlc.arg(email), sqlc.arg(phone), sqlc.arg(iban), b.id
FROM budgets b WHERE b.id = sqlc.arg(budget_id);

-- name: ListSuppliers :many
SELECT s.id, s.identity_number, s.name, s.description, s.email, s.phone, s.iban,
       s.budget_id, b.name AS budget_name
FROM suppliers s JOIN budgets b ON b.id = s.budget_id
ORDER BY s.name, s.id;

-- name: UpdateSupplier :execrows
UPDATE suppliers
SET identity_number = sqlc.arg(identity_number), name = sqlc.arg(supplier_name),
    description = sqlc.arg(description), email = sqlc.arg(email),
    phone = sqlc.arg(phone), iban = sqlc.arg(iban), budget_id = sqlc.arg(budget_id)
WHERE suppliers.id = sqlc.arg(supplier_id)
  AND EXISTS (SELECT 1 FROM budgets WHERE budgets.id = sqlc.arg(budget_id));

-- name: SupplierMatches :one
SELECT COUNT(*) FROM suppliers
WHERE id = sqlc.arg(supplier_id) AND budget_id = sqlc.arg(budget_id)
  AND BINARY identity_number = sqlc.arg(identity_number) AND BINARY name = sqlc.arg(supplier_name)
  AND BINARY description = sqlc.arg(description) AND BINARY email = sqlc.arg(email)
  AND BINARY phone = sqlc.arg(phone) AND BINARY iban = sqlc.arg(iban);
