-- name: TakeRateLimit :exec
INSERT INTO rate_limits (scope, bucket_key, started_at, max_requests, requests, expires_at)
VALUES (sqlc.arg(scope), sqlc.arg(bucket_key), sqlc.arg(started_at), sqlc.arg(max_requests), 1, sqlc.arg(expires_at))
ON DUPLICATE KEY UPDATE
  requests = CASE
    WHEN rate_limits.expires_at <= VALUES(started_at) THEN 1
    WHEN rate_limits.requests <= rate_limits.max_requests THEN rate_limits.requests + 1
    ELSE rate_limits.requests
  END,
  max_requests = CASE
    WHEN rate_limits.expires_at <= VALUES(started_at) THEN VALUES(max_requests)
    ELSE rate_limits.max_requests
  END,
  started_at = CASE
    WHEN rate_limits.expires_at <= VALUES(started_at) THEN VALUES(started_at)
    ELSE rate_limits.started_at
  END,
  expires_at = CASE
    WHEN rate_limits.expires_at <= VALUES(started_at) THEN VALUES(expires_at)
    ELSE rate_limits.expires_at
  END;

-- name: SelectRateLimit :one
SELECT requests, max_requests, expires_at
FROM rate_limits
WHERE scope = ? AND bucket_key = ?;

-- name: DeleteExpiredRateLimits :execrows
DELETE FROM rate_limits
WHERE expires_at <= sqlc.arg(now)
ORDER BY expires_at
LIMIT 100;
