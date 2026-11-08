CREATE TABLE IF NOT EXISTS account_email_change_requests (
  sub BIGINT PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  otp VARCHAR(6) NOT NULL,
  expires_at BIGINT NOT NULL,

  CONSTRAINT account_email_change_requests_sub_fk FOREIGN KEY (sub) REFERENCES accounts(sub) ON DELETE CASCADE,
  CONSTRAINT account_email_change_requests_email_len CHECK (CHAR_LENGTH(email) <= 255),
  CONSTRAINT account_email_change_requests_otp_len CHECK (CHAR_LENGTH(otp) = 6),
  CONSTRAINT account_email_change_requests_otp_digits CHECK (otp REGEXP '^[0-9]{6}$')
);
