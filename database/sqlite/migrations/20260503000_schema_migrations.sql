CREATE TABLE IF NOT EXISTS schema_migrations (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  version INTEGER NOT NULL
);
