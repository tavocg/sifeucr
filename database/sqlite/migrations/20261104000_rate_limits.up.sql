CREATE TABLE rate_limits (
  scope TEXT NOT NULL,
  bucket_key TEXT NOT NULL,
  started_at INTEGER NOT NULL,
  max_requests INTEGER NOT NULL CHECK (max_requests > 0),
  requests INTEGER NOT NULL CHECK (requests > 0),
  expires_at INTEGER NOT NULL,
  PRIMARY KEY (scope, bucket_key)
);

CREATE INDEX rate_limits_expires_at_idx ON rate_limits(expires_at);
