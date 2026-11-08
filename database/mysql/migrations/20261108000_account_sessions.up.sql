-- Preserve existing refresh tokens as sessions with unknown device information.
ALTER TABLE refresh_tokens
  ADD COLUMN session_id VARCHAR(64) NULL,
  ADD COLUMN user_agent VARCHAR(512) NOT NULL DEFAULT '',
  ADD COLUMN last_used_at BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN access_expires_at BIGINT NOT NULL DEFAULT 0;

UPDATE refresh_tokens SET session_id = LOWER(SHA2(CONCAT('session:', token_hash), 256)), last_used_at = created_at;
ALTER TABLE refresh_tokens MODIFY COLUMN session_id VARCHAR(64) NOT NULL;

CREATE UNIQUE INDEX refresh_tokens_session_id_idx ON refresh_tokens(session_id);
CREATE INDEX refresh_tokens_sessions_idx ON refresh_tokens(sub, created_at, session_id);
