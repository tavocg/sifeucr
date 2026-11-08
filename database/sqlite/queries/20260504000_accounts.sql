-- name: EnsureAccountByEmail :one
INSERT INTO accounts (sub, email, created_at, auth_id)
VALUES ((SELECT last_sub + 1 FROM account_subject_sequence WHERE id = 1), ?, ?, ?)
ON CONFLICT(email) DO UPDATE SET email = excluded.email
RETURNING sub;

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
UPDATE accounts
SET auth_id = auth_id
WHERE sub = ?
RETURNING auth_id;

-- name: SetAccountAuthID :exec
UPDATE accounts
SET auth_id = ?
WHERE sub = ?;
