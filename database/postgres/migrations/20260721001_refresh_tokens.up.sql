CREATE TABLE IF NOT EXISTS refresh_tokens (
  token_hash VARCHAR(64) PRIMARY KEY,
  sub BIGINT NOT NULL REFERENCES accounts(sub) ON DELETE CASCADE,
  expires_at BIGINT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM NOW())::BIGINT)
);

CREATE INDEX IF NOT EXISTS refresh_tokens_sub_idx ON refresh_tokens(sub);
