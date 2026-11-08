CREATE TABLE IF NOT EXISTS schema_migrations (
  id SMALLINT PRIMARY KEY,
  version BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_migration_pending (
  id SMALLINT PRIMARY KEY,
  version BIGINT NOT NULL
);
