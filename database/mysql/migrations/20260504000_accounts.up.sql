CREATE TABLE IF NOT EXISTS accounts (
  sub BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  created_at BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),

  CONSTRAINT accounts_email_len CHECK (CHAR_LENGTH(email) <= 255)
);

CREATE OR REPLACE VIEW accounts_meta AS
SELECT
  a.sub,
  a.email,
  a.created_at
FROM accounts a;
