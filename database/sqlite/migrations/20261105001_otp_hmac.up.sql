-- SQLite cannot drop CHECK constraints, so rebuild both challenge tables.
-- Pending plaintext challenges are discarded because they cannot be verified
-- against keyed digests.
DELETE FROM account_login_requests;
DELETE FROM account_email_change_requests;

CREATE TABLE account_login_requests_hmac (
  email VARCHAR(255) PRIMARY KEY NOT NULL,
  otp_hash VARCHAR(64) NOT NULL,
  expires_at INTEGER NOT NULL,

  CONSTRAINT account_login_requests_email_len CHECK (length(email) <= 255),
  CONSTRAINT account_login_requests_otp_hash_len CHECK (length(otp_hash) = 64),
  CONSTRAINT account_login_requests_otp_hash_hex CHECK (otp_hash NOT GLOB '*[^0-9a-f]*')
);

DROP TABLE account_login_requests;
ALTER TABLE account_login_requests_hmac RENAME TO account_login_requests;
CREATE INDEX account_login_requests_expires_at_idx ON account_login_requests(expires_at);

CREATE TABLE account_email_change_requests_hmac (
  sub INTEGER PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  otp_hash VARCHAR(64) NOT NULL,
  expires_at INTEGER NOT NULL,

  CONSTRAINT account_email_change_requests_email_len CHECK (length(email) <= 255),
  CONSTRAINT account_email_change_requests_otp_hash_len CHECK (length(otp_hash) = 64),
  CONSTRAINT account_email_change_requests_otp_hash_hex CHECK (otp_hash NOT GLOB '*[^0-9a-f]*')
);

DROP TABLE account_email_change_requests;
ALTER TABLE account_email_change_requests_hmac RENAME TO account_email_change_requests;
CREATE INDEX account_email_change_requests_expires_at_idx ON account_email_change_requests(expires_at);
