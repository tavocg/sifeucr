CREATE TABLE IF NOT EXISTS refresh_tokens (
  token_hash VARCHAR(64) PRIMARY KEY,
  sub INTEGER NOT NULL REFERENCES accounts(sub) ON DELETE CASCADE,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch('now'))
);

CREATE INDEX IF NOT EXISTS refresh_tokens_sub_idx ON refresh_tokens(sub);
