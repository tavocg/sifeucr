-- Pending plaintext challenges cannot be verified once codes are keyed digests.
DELETE FROM account_login_requests;
DELETE FROM account_email_change_requests;

ALTER TABLE account_login_requests DROP CONSTRAINT IF EXISTS account_login_requests_otp_len;
ALTER TABLE account_login_requests DROP CONSTRAINT IF EXISTS account_login_requests_otp_digits;
ALTER TABLE account_login_requests RENAME COLUMN otp TO otp_hash;
ALTER TABLE account_login_requests ALTER COLUMN otp_hash TYPE VARCHAR(64);
ALTER TABLE account_login_requests ADD CONSTRAINT account_login_requests_otp_hash_len CHECK (length(otp_hash) = 64);
ALTER TABLE account_login_requests ADD CONSTRAINT account_login_requests_otp_hash_hex CHECK (otp_hash ~ '^[0-9a-f]{64}$');

ALTER TABLE account_email_change_requests DROP CONSTRAINT IF EXISTS account_email_change_requests_otp_len;
ALTER TABLE account_email_change_requests DROP CONSTRAINT IF EXISTS account_email_change_requests_otp_digits;
ALTER TABLE account_email_change_requests RENAME COLUMN otp TO otp_hash;
ALTER TABLE account_email_change_requests ALTER COLUMN otp_hash TYPE VARCHAR(64);
ALTER TABLE account_email_change_requests ADD CONSTRAINT account_email_change_requests_otp_hash_len CHECK (length(otp_hash) = 64);
ALTER TABLE account_email_change_requests ADD CONSTRAINT account_email_change_requests_otp_hash_hex CHECK (otp_hash ~ '^[0-9a-f]{64}$');
