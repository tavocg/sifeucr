-- Retain only hashes and session identities, never refresh credentials.
CREATE TABLE consumed_refresh_tokens (
  token_hash VARCHAR(64) PRIMARY KEY,
  sub BIGINT NOT NULL,
  auth_id VARCHAR(64) NOT NULL,
  session_id VARCHAR(64) NOT NULL,
  expires_at BIGINT NOT NULL,
  FOREIGN KEY (sub) REFERENCES accounts(sub) ON DELETE CASCADE
);
CREATE INDEX consumed_refresh_tokens_expires_idx ON consumed_refresh_tokens(expires_at);
