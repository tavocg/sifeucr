-- Bind each refresh token to the account session generation that issued it.
ALTER TABLE refresh_tokens ADD COLUMN auth_id TEXT;

UPDATE refresh_tokens
SET auth_id = accounts.auth_id
FROM accounts
WHERE refresh_tokens.sub = accounts.sub;

ALTER TABLE refresh_tokens ALTER COLUMN auth_id SET NOT NULL;
