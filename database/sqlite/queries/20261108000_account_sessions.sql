-- name: SelectSessionAuthID :one
SELECT accounts.auth_id
FROM accounts
JOIN refresh_tokens ON refresh_tokens.sub = accounts.sub AND refresh_tokens.auth_id = accounts.auth_id
WHERE accounts.sub = sqlc.arg(sub) AND refresh_tokens.session_id = sqlc.arg(session_id);

-- name: SelectAccountSessions :many
SELECT session_id, user_agent, created_at, last_used_at, expires_at, access_expires_at
FROM refresh_tokens
WHERE sub = sqlc.arg(sub)
  AND (expires_at = 0 OR expires_at > sqlc.arg(now) OR access_expires_at > sqlc.arg(now))
  AND (created_at < sqlc.arg(before_created_at) OR (created_at = sqlc.arg(before_created_at) AND session_id < sqlc.arg(before_id)))
ORDER BY created_at DESC, session_id DESC
LIMIT 101;

-- name: DeleteAccountSession :exec
DELETE FROM refresh_tokens WHERE sub = sqlc.arg(sub) AND session_id = sqlc.arg(session_id);
