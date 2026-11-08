CREATE TABLE rate_limits (
  scope VARCHAR(128) NOT NULL,
  bucket_key VARCHAR(255) NOT NULL,
  started_at BIGINT NOT NULL,
  max_requests BIGINT NOT NULL CHECK (max_requests > 0),
  requests BIGINT NOT NULL CHECK (requests > 0),
  expires_at BIGINT NOT NULL,
  PRIMARY KEY (scope, bucket_key)
);

CREATE INDEX rate_limits_expires_at_idx ON rate_limits(expires_at);
