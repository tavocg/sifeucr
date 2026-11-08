-- name: UpsertAccountEmailChangeRequest :exec
INSERT INTO account_email_change_requests (sub, email, otp_hash, expires_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(sub) DO UPDATE SET
  email = excluded.email,
  otp_hash = excluded.otp_hash,
  expires_at = excluded.expires_at;

-- name: ConsumeAccountEmailChangeRequest :one
DELETE FROM account_email_change_requests
WHERE sub = ? AND otp_hash = ? AND expires_at > ?
RETURNING sub, email, otp_hash, expires_at;

-- name: DeleteAccountEmailChangeRequest :exec
DELETE FROM account_email_change_requests
WHERE sub = ?;

-- name: DeleteExpiredEmailChangeRequests :exec
DELETE FROM account_email_change_requests
WHERE account_email_change_requests.expires_at <= sqlc.arg(now)
  AND sub IN (
    SELECT sub FROM account_email_change_requests
    WHERE account_email_change_requests.expires_at <= sqlc.arg(now)
    ORDER BY expires_at
    LIMIT 1000
  );
