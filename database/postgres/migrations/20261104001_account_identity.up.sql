ALTER TABLE accounts ADD COLUMN auth_id TEXT NOT NULL DEFAULT gen_random_uuid()::text;
CREATE UNIQUE INDEX accounts_auth_id_idx ON accounts(auth_id);

-- Require fresh authentication when migrating from subject-only sessions.
DELETE FROM refresh_tokens;
