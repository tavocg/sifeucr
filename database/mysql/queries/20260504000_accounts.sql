-- name: EnsureAccountByEmail :exec
INSERT INTO accounts (email, created_at, auth_id)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE email = VALUES(email);

-- name: SelectAccountSubByEmail :one
SELECT sub FROM accounts WHERE email = ?;

-- name: UpdateAccountEmail :exec
UPDATE accounts
SET email = ?
WHERE sub = ?;

-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE sub = ?;

-- name: SelectAccountBySub :one
SELECT sub, email, created_at
FROM accounts
WHERE sub = ?;

-- name: SelectAccountAuthID :one
SELECT auth_id FROM accounts WHERE sub = ?;

-- name: LockAccountAuthID :one
SELECT auth_id FROM accounts WHERE sub = ? FOR UPDATE;

-- name: SetAccountAuthID :exec
UPDATE accounts
SET auth_id = ?
WHERE sub = ?;
