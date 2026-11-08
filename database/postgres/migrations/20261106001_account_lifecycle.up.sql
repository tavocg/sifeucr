-- Match account-owned challenge cleanup to roles and refresh tokens.
ALTER TABLE account_email_change_requests
  DROP CONSTRAINT account_email_change_requests_sub_fkey;
ALTER TABLE account_email_change_requests
  ADD CONSTRAINT account_email_change_requests_sub_fk
  FOREIGN KEY (sub) REFERENCES accounts(sub) ON DELETE CASCADE;

DROP VIEW accounts_meta;
