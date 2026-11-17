-- name: InsertConsumedRefreshToken :exec
INSERT INTO consumed_refresh_tokens (token_hash, sub, auth_id, session_id, expires_at)
VALUES (sqlc.arg(token_hash), sqlc.arg(sub), sqlc.arg(auth_id), sqlc.arg(session_id), sqlc.arg(expires_at));

-- name: SelectRevocableSessionByHash :one
SELECT r.sub, r.auth_id, r.session_id FROM refresh_tokens r
WHERE r.token_hash = sqlc.arg(token_hash)
UNION ALL
SELECT c.sub, c.auth_id, c.session_id FROM consumed_refresh_tokens c
WHERE c.token_hash = sqlc.arg(token_hash) AND c.expires_at > sqlc.arg(now)
LIMIT 1;

-- name: DeleteExpiredConsumedRefreshTokens :execrows
DELETE FROM consumed_refresh_tokens
WHERE token_hash IN (
  SELECT c.token_hash FROM consumed_refresh_tokens c
  WHERE c.expires_at <= sqlc.arg(now)
  ORDER BY c.expires_at LIMIT 100
);
