-- Preserve account-owned pending challenges and discard orphaned challenges.
CREATE TABLE account_email_change_requests_owned (
  sub INTEGER PRIMARY KEY REFERENCES accounts(sub) ON DELETE CASCADE,
  email VARCHAR(255) NOT NULL,
  otp_hash VARCHAR(64) NOT NULL,
  expires_at INTEGER NOT NULL,

  CONSTRAINT account_email_change_requests_email_len CHECK (length(email) <= 255),
  CONSTRAINT account_email_change_requests_otp_hash_len CHECK (length(otp_hash) = 64),
  CONSTRAINT account_email_change_requests_otp_hash_hex CHECK (otp_hash NOT GLOB '*[^0-9a-f]*')
);

INSERT INTO account_email_change_requests_owned (sub, email, otp_hash, expires_at)
SELECT r.sub, r.email, r.otp_hash, r.expires_at
FROM account_email_change_requests r
JOIN accounts a ON a.sub = r.sub;

DROP TABLE account_email_change_requests;
ALTER TABLE account_email_change_requests_owned RENAME TO account_email_change_requests;
CREATE INDEX account_email_change_requests_expires_at_idx ON account_email_change_requests(expires_at);

DROP VIEW accounts_meta;
