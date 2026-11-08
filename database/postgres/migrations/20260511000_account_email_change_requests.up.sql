CREATE TABLE IF NOT EXISTS account_email_change_requests (
  sub BIGINT PRIMARY KEY REFERENCES accounts(sub),
  email VARCHAR(255) NOT NULL,
  otp VARCHAR(6) NOT NULL,
  expires_at BIGINT NOT NULL,

  CONSTRAINT account_email_change_requests_email_len CHECK (length(email) <= 255),
  CONSTRAINT account_email_change_requests_otp_len CHECK (length(otp) = 6),
  CONSTRAINT account_email_change_requests_otp_digits CHECK (otp ~ '^[0-9]{6}$')
);
