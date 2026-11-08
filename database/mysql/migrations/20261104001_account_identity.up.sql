ALTER TABLE accounts ADD COLUMN auth_id VARCHAR(36) NULL;
UPDATE accounts SET auth_id = UUID() WHERE auth_id IS NULL;
ALTER TABLE accounts MODIFY COLUMN auth_id VARCHAR(36) NOT NULL DEFAULT (UUID());
CREATE UNIQUE INDEX accounts_auth_id_idx ON accounts(auth_id);

-- Require fresh authentication when migrating from subject-only sessions.
DELETE FROM refresh_tokens;
