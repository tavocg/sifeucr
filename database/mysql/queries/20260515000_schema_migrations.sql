-- name: GetSchemaMigrationVersion :one
SELECT CAST(COALESCE((SELECT version FROM schema_migrations WHERE id = 1), 0) AS SIGNED);

-- name: SetSchemaMigrationVersion :exec
INSERT INTO schema_migrations (id, version)
VALUES (1, ?)
ON DUPLICATE KEY UPDATE version = VALUES(version);

-- name: GetPendingSchemaMigration :one
SELECT CAST(COALESCE((SELECT version FROM schema_migration_pending WHERE id = 1), 0) AS SIGNED);

-- name: SetPendingSchemaMigration :exec
INSERT INTO schema_migration_pending (id, version) VALUES (1, ?);

-- name: ClearPendingSchemaMigration :exec
DELETE FROM schema_migration_pending WHERE id = 1;
