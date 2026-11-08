-- name: UpsertAccountLoginRequest :exec
INSERT INTO account_login_requests (email, otp_hash, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (email) DO UPDATE SET
  otp_hash = EXCLUDED.otp_hash,
  expires_at = EXCLUDED.expires_at;

-- name: ConsumeAccountLoginRequest :one
DELETE FROM account_login_requests
WHERE email = $1 AND otp_hash = $2 AND expires_at > $3
RETURNING email, otp_hash, expires_at;

-- name: DeleteAccountLoginRequest :exec
DELETE FROM account_login_requests
WHERE email = $1;

-- name: DeleteExpiredLoginRequests :exec
DELETE FROM account_login_requests
WHERE account_login_requests.expires_at <= sqlc.arg(now)
  AND email IN (
    SELECT email FROM account_login_requests
    WHERE account_login_requests.expires_at <= sqlc.arg(now)
    ORDER BY expires_at
    LIMIT 1000
  );
