CREATE TABLE IF NOT EXISTS account_login_requests (
  email VARCHAR(255) PRIMARY KEY NOT NULL,
  otp VARCHAR(6) NOT NULL,
  expires_at BIGINT NOT NULL,

  CONSTRAINT account_login_requests_email_len CHECK (CHAR_LENGTH(email) <= 255),
  CONSTRAINT account_login_requests_otp_len CHECK (CHAR_LENGTH(otp) = 6),
  CONSTRAINT account_login_requests_otp_digits CHECK (otp REGEXP '^[0-9]{6}$')
);
