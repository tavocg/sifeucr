-- name: UpsertAccountEmailChangeRequest :exec
INSERT INTO account_email_change_requests (sub, email, otp_hash, expires_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (sub) DO UPDATE SET
  email = EXCLUDED.email,
  otp_hash = EXCLUDED.otp_hash,
  expires_at = EXCLUDED.expires_at;

-- name: ConsumeAccountEmailChangeRequest :one
DELETE FROM account_email_change_requests
WHERE sub = $1 AND otp_hash = $2 AND expires_at > $3
RETURNING sub, email, otp_hash, expires_at;

-- name: DeleteAccountEmailChangeRequest :exec
DELETE FROM account_email_change_requests
WHERE sub = $1;

-- name: DeleteExpiredEmailChangeRequests :exec
DELETE FROM account_email_change_requests
WHERE account_email_change_requests.expires_at <= sqlc.arg(now)
  AND sub IN (
    SELECT sub FROM account_email_change_requests
    WHERE account_email_change_requests.expires_at <= sqlc.arg(now)
    ORDER BY expires_at
    LIMIT 1000
  );
