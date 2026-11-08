-- name: UpsertAccountEmailChangeRequest :exec
INSERT INTO account_email_change_requests (sub, email, otp_hash, expires_at)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  email = VALUES(email),
  otp_hash = VALUES(otp_hash),
  expires_at = VALUES(expires_at);

-- name: SelectAccountEmailChangeRequestForUpdate :one
SELECT sub, email, otp_hash, expires_at
FROM account_email_change_requests
WHERE sub = ? AND otp_hash = ? AND expires_at > ?
FOR UPDATE;

-- name: DeleteAccountEmailChangeRequest :exec
DELETE FROM account_email_change_requests
WHERE sub = ?;

-- name: DeleteExpiredEmailChangeRequests :exec
DELETE FROM account_email_change_requests
WHERE account_email_change_requests.expires_at <= sqlc.arg(now)
  AND sub IN (
    SELECT sub FROM (
      SELECT sub FROM account_email_change_requests
      WHERE account_email_change_requests.expires_at <= sqlc.arg(now)
      ORDER BY expires_at
      LIMIT 1000
    ) AS expired_email_change_requests
  );
