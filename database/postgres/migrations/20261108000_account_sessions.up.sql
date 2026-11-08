-- Preserve existing refresh tokens as sessions with unknown device information.
ALTER TABLE refresh_tokens ADD COLUMN session_id VARCHAR(64);
ALTER TABLE refresh_tokens ADD COLUMN user_agent TEXT NOT NULL DEFAULT '';
ALTER TABLE refresh_tokens ADD COLUMN last_used_at BIGINT NOT NULL DEFAULT 0;
ALTER TABLE refresh_tokens ADD COLUMN access_expires_at BIGINT NOT NULL DEFAULT 0;

UPDATE refresh_tokens SET session_id = encode(sha256(convert_to('session:' || token_hash, 'UTF8')), 'hex'), last_used_at = created_at;
ALTER TABLE refresh_tokens ALTER COLUMN session_id SET NOT NULL;

CREATE UNIQUE INDEX refresh_tokens_session_id_idx ON refresh_tokens(session_id);
CREATE INDEX refresh_tokens_sessions_idx ON refresh_tokens(sub, created_at, session_id);
