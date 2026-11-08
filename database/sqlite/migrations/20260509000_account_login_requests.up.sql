CREATE TABLE IF NOT EXISTS account_login_requests (
  email VARCHAR(255) PRIMARY KEY NOT NULL,
  otp VARCHAR(6) NOT NULL,
  expires_at INTEGER NOT NULL,

  CONSTRAINT account_login_requests_email_len CHECK (length(email) <= 255),
  CONSTRAINT account_login_requests_otp_len CHECK (length(otp) = 6),
  CONSTRAINT account_login_requests_otp_digits CHECK (otp GLOB '[0-9][0-9][0-9][0-9][0-9][0-9]')
);
