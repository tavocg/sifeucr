-- Bind each refresh token to the account session generation that issued it.
ALTER TABLE refresh_tokens ADD COLUMN auth_id TEXT NOT NULL DEFAULT '';

UPDATE refresh_tokens
SET auth_id = (SELECT accounts.auth_id FROM accounts WHERE accounts.sub = refresh_tokens.sub);
