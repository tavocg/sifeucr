CREATE TABLE IF NOT EXISTS accounts (
  sub INTEGER PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  created_at INTEGER NOT NULL DEFAULT (unixepoch('now')),

  CONSTRAINT accounts_email_len CHECK (length(email) <= 255)
);

CREATE VIEW IF NOT EXISTS accounts_meta AS
SELECT
  a.*
FROM accounts a;
