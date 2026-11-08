-- name: EnsureAccountByEmail :one
INSERT INTO accounts (email, created_at, auth_id)
VALUES ($1, $2, $3)
ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
RETURNING sub;

-- name: UpdateAccountEmail :exec
UPDATE accounts
SET email = $2
WHERE sub = $1;

-- name: DeleteAccount :exec
DELETE FROM accounts
WHERE sub = $1;

-- name: SelectAccountBySub :one
SELECT sub, email, created_at
FROM accounts
WHERE sub = $1;

-- name: SelectAccountAuthID :one
SELECT auth_id FROM accounts WHERE sub = $1;

-- name: LockAccountAuthID :one
SELECT auth_id FROM accounts WHERE sub = $1 FOR UPDATE;

-- name: SetAccountAuthID :exec
UPDATE accounts
SET auth_id = $2
WHERE sub = $1;
