# Separate migration and runtime credentials

Use a schema-changing account only for `sifeucr migrate`. Start the server with a
different SQL account and `--auto-migrate=false` after migration succeeds:

```sh
SIFEUCR_DB="$MIGRATION_DSN" sifeucr migrate
SIFEUCR_DB="$RUNTIME_DSN" sifeucr serve --auto-migrate=false
```

Inject these variables through the deployment's secret store. Do not pass the
migration account to server replicas. The migration command does not need JWT,
SMTP credentials. A failed migration must prevent rollout; never clear a
MySQL pending marker to bypass failure. See [MySQL recovery](../AGENTS.md#mysql-migration-recovery).

The runtime needs application DML and maintenance deletes, plus read access to
schema-version metadata. It must not own the schema or have DDL/grant privileges.
Schema checking requires the exact embedded version and rejects unfinished
MySQL migrations. Automatic migration remains enabled by default for local use.

## MySQL grants

Create the database and accounts as an administrator. Replace `sifeucr`, passwords
and private host patterns below with deployment-specific values. Restrict account
hosts to the sifeucr/job's private network instead of exposing accounts publicly.
The following grants match the current template; review them when adding tables.

```sql
CREATE USER 'sifeucr_migrate'@'<private-job-host>' IDENTIFIED BY '<migration-secret>';
CREATE USER 'sifeucr_runtime'@'<private-sifeucr-host>' IDENTIFIED BY '<runtime-secret>';

GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX,
      REFERENCES, CREATE VIEW, SHOW VIEW
ON sifeucr.* TO 'sifeucr_migrate'@'<private-job-host>';

GRANT SELECT, INSERT, UPDATE, DELETE ON sifeucr.accounts TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT, UPDATE, DELETE ON sifeucr.refresh_tokens TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT, UPDATE, DELETE ON sifeucr.consumed_refresh_tokens TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT, UPDATE, DELETE ON sifeucr.rate_limits TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT, UPDATE ON sifeucr.account_roles TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, DELETE ON sifeucr.pending_setup TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT, UPDATE ON sifeucr.suppliers TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT ON sifeucr.budgets TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT, INSERT ON sifeucr.purchase_processes TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT ON sifeucr.roles TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT ON sifeucr.permissions TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT ON sifeucr.role_permissions TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT ON sifeucr.schema_migrations TO 'sifeucr_runtime'@'<private-sifeucr-host>';
GRANT SELECT ON sifeucr.schema_migration_pending TO 'sifeucr_runtime'@'<private-sifeucr-host>';
```

Apply runtime grants after tables exist. Keep schema metadata read-only and
reference roles/permissions read-only; `account_roles` needs INSERT/UPDATE because
login assigns the default role through an idempotent upsert. Account deletion uses
foreign-key cascades. New backend features may require additional explicit grants,
not broad schema ownership.

MySQL migration files run on a temporary connection with multiple statements
enabled. It closes before the normal pool opens. The runtime pool disables that
capability even if the DSN contains `multiStatements=true`; it is never enabled
with `--auto-migrate=false`. Queries still use parameters. This reduces exposed
capabilities and does not replace input validation or parameterized SQL.

## Other backends

PostgreSQL also needs separate roles: the runtime must not own tables or inherit
the migration role. Grant access to the application schema, application DML,
required identity sequences and SELECT on schema metadata; apply grants to future
objects created by the migration owner as well. Avoid granting DML on metadata
when granting permissions across all tables. See the [replica deployment guide](multi-instance.md).

SQLite has no SQL account/grant system. Its boundary is the service-owned database
directory and private file permissions. Keep that storage inaccessible to other
users; the sifeucr requires write access to the database and its journal/WAL files.
