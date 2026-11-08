-- Pending plaintext challenges cannot be verified once codes are keyed digests.
DELETE FROM account_login_requests;
DELETE FROM account_email_change_requests;

ALTER TABLE account_login_requests DROP CONSTRAINT account_login_requests_otp_len;
ALTER TABLE account_login_requests DROP CONSTRAINT account_login_requests_otp_digits;
ALTER TABLE account_login_requests CHANGE COLUMN otp otp_hash VARCHAR(64) NOT NULL;
ALTER TABLE account_login_requests ADD CONSTRAINT account_login_requests_otp_hash_len CHECK (CHAR_LENGTH(otp_hash) = 64);
ALTER TABLE account_login_requests ADD CONSTRAINT account_login_requests_otp_hash_hex CHECK (otp_hash REGEXP '^[0-9a-f]{64}$');

ALTER TABLE account_email_change_requests DROP CONSTRAINT account_email_change_requests_otp_len;
ALTER TABLE account_email_change_requests DROP CONSTRAINT account_email_change_requests_otp_digits;
ALTER TABLE account_email_change_requests CHANGE COLUMN otp otp_hash VARCHAR(64) NOT NULL;
ALTER TABLE account_email_change_requests ADD CONSTRAINT account_email_change_requests_otp_hash_len CHECK (CHAR_LENGTH(otp_hash) = 64);
ALTER TABLE account_email_change_requests ADD CONSTRAINT account_email_change_requests_otp_hash_hex CHECK (otp_hash REGEXP '^[0-9a-f]{64}$');
