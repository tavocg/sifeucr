-- name: TakeRateLimit :one
INSERT INTO rate_limits (scope, bucket_key, started_at, max_requests, requests, expires_at)
VALUES (sqlc.arg(scope), sqlc.arg(bucket_key), sqlc.arg(started_at), sqlc.arg(max_requests), 1, sqlc.arg(expires_at))
ON CONFLICT (scope, bucket_key) DO UPDATE SET
  requests = CASE
    WHEN rate_limits.expires_at <= excluded.started_at THEN 1
    WHEN rate_limits.requests <= rate_limits.max_requests THEN rate_limits.requests + 1
    ELSE rate_limits.requests
  END,
  max_requests = CASE
    WHEN rate_limits.expires_at <= excluded.started_at THEN excluded.max_requests
    ELSE rate_limits.max_requests
  END,
  started_at = CASE
    WHEN rate_limits.expires_at <= excluded.started_at THEN excluded.started_at
    ELSE rate_limits.started_at
  END,
  expires_at = CASE
    WHEN rate_limits.expires_at <= excluded.started_at THEN excluded.expires_at
    ELSE rate_limits.expires_at
  END
RETURNING requests, max_requests, expires_at;

-- name: DeleteExpiredRateLimits :execrows
DELETE FROM rate_limits
WHERE rate_limits.expires_at <= sqlc.arg(now)
  AND (rate_limits.scope, rate_limits.bucket_key) IN (
    SELECT scope, bucket_key FROM rate_limits
    WHERE rate_limits.expires_at <= sqlc.arg(now)
    ORDER BY expires_at
    LIMIT 100
  );
