CREATE INDEX account_login_requests_expires_at_idx ON account_login_requests(expires_at);
CREATE INDEX account_email_change_requests_expires_at_idx ON account_email_change_requests(expires_at);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens(expires_at) WHERE expires_at != 0;
