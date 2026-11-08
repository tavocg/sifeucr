-- Bind each refresh token to the account session generation that issued it.
ALTER TABLE refresh_tokens ADD COLUMN auth_id VARCHAR(36) NULL;

UPDATE refresh_tokens
JOIN accounts ON accounts.sub = refresh_tokens.sub
SET refresh_tokens.auth_id = accounts.auth_id;

ALTER TABLE refresh_tokens MODIFY COLUMN auth_id VARCHAR(36) NOT NULL;
