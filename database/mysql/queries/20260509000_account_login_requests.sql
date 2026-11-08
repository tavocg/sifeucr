-- name: UpsertAccountLoginRequest :exec
INSERT INTO account_login_requests (email, otp_hash, expires_at)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE
  otp_hash = VALUES(otp_hash),
  expires_at = VALUES(expires_at);

-- name: SelectAccountLoginRequestForUpdate :one
SELECT email, otp_hash, expires_at
FROM account_login_requests
WHERE email = ? AND otp_hash = ? AND expires_at > ?
FOR UPDATE;

-- name: DeleteAccountLoginRequest :exec
DELETE FROM account_login_requests
WHERE email = ?;

-- name: DeleteExpiredLoginRequests :exec
DELETE FROM account_login_requests
WHERE account_login_requests.expires_at <= sqlc.arg(now)
  AND email IN (
    SELECT email FROM (
      SELECT email FROM account_login_requests
      WHERE account_login_requests.expires_at <= sqlc.arg(now)
      ORDER BY expires_at
      LIMIT 1000
    ) AS expired_login_requests
  );
