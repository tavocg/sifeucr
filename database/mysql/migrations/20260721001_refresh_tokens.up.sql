CREATE TABLE IF NOT EXISTS refresh_tokens (
  token_hash VARCHAR(64) PRIMARY KEY,
  sub BIGINT NOT NULL,
  expires_at BIGINT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),

  CONSTRAINT refresh_tokens_account_fk FOREIGN KEY (sub) REFERENCES accounts(sub) ON DELETE CASCADE
);

CREATE INDEX refresh_tokens_sub_idx ON refresh_tokens(sub);
