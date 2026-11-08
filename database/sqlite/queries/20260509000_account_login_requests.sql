-- name: UpsertAccountLoginRequest :exec
INSERT INTO account_login_requests (email, otp_hash, expires_at)
VALUES (?, ?, ?)
ON CONFLICT(email) DO UPDATE SET
  otp_hash = excluded.otp_hash,
  expires_at = excluded.expires_at;

-- name: ConsumeAccountLoginRequest :one
DELETE FROM account_login_requests
WHERE email = ? AND otp_hash = ? AND expires_at > ?
RETURNING email, otp_hash, expires_at;

-- name: DeleteAccountLoginRequest :exec
DELETE FROM account_login_requests
WHERE email = ?;

-- name: DeleteExpiredLoginRequests :exec
DELETE FROM account_login_requests
WHERE account_login_requests.expires_at <= sqlc.arg(now)
  AND email IN (
    SELECT email FROM account_login_requests
    WHERE account_login_requests.expires_at <= sqlc.arg(now)
    ORDER BY expires_at
    LIMIT 1000
  );
