-- New application writes provide the same random generation on every backend.
-- Preserve existing identities and refresh tokens so deployments retain sessions.
ALTER TABLE accounts ALTER COLUMN auth_id DROP DEFAULT;
