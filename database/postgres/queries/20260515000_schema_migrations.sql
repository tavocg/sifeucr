-- name: GetSchemaMigrationVersion :one
SELECT COALESCE((SELECT version FROM schema_migrations WHERE id = 1), 0)::BIGINT;

-- name: SetSchemaMigrationVersion :exec
INSERT INTO schema_migrations (id, version)
VALUES (1, $1)
ON CONFLICT (id) DO UPDATE SET version = EXCLUDED.version;
