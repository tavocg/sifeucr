# Multiple instances and Kubernetes

Run HTTP replicas against the **same PostgreSQL or MySQL database**, with identical
JWT secrets and institutional SMTP configuration and token TTLs. No session affinity or shared filesystem is needed.
Authentication, revocation, permissions and identity quotas use
SQL. Keep these operations on the authoritative database, not lagging read replicas.
SQLite remains supported for a single-instance installation.

## Runtime changes

| Setting / flag | Default | Purpose |
| --- | --- | --- |
| `database.auto-migrate` / `--auto-migrate` | `true` | Keep local startup convenient; set false for replicas. |
| `database.max-connections` / `--db-max-connections` | `10` | Per-process PostgreSQL/MySQL pool ceiling; SQLite uses one. |
| `cache.enabled` / `--cache` | `false` | Optional local profile cache with no cross-process invalidation. |
| `http.drain-delay` / `--drain-delay` | `5s` | Continue serving after readiness withdrawal so routing can converge; skipped in `--dev`. |
| `http.shutdown-timeout` / `--shutdown-timeout` | `25s` | Finish active requests after closing listeners. |
| `auth.smtp.address` / `--auth-smtp-address` | empty | Institutional SMTP host:port; required outside development. |
| `auth.smtp.starttls` / `--auth-smtp-starttls` | `false` | Require STARTTLS instead of implicit TLS. |

Environment names replace dots/hyphens with underscores and add `SIFEUCR_`, for
example `SIFEUCR_DATABASE_AUTO_MIGRATE=false`. The existing `SIFEUCR_DB` remains the DSN.
No plaintext SMTP or insecure TLS mode is added. Use a trusted CA (custom CAs can be mounted and selected with `SSL_CERT_FILE`).

The optional cache only invalidates locally; its 20-second TTL starts on cache
insertion. Keep it disabled for immediate profile visibility across replicas.
Adding a shared store alone would not distribute its process-local generations.

## Image and release sequence

Build with `docker build --target sifeucr -t sifeucr:multi-instance .`. The multistage Dockerfile
regenerates sqlc, templ and API documentation and produces a non-root static
binary with CA certificates. Node and Go are absent from the runtime image.
Use an immutable registry tag/digest for a real release, replacing the image in
both Kubernetes manifests. Publishing requires your registry's credentials;
this repository does not assume a registry or publish credentials.

Create two Kubernetes Secrets from protected environment files:

- `sifeucr-runtime`: `SIFEUCR_DB` (runtime SQL credentials), `SIFEUCR_AUTH_SECRET`,
  `SIFEUCR_AUTH_SMTP_ADDRESS` and optionally `SIFEUCR_AUTH_SMTP_STARTTLS`.
- `sifeucr-migrations`: `SIFEUCR_DB` with schema-changing privileges. No JWT or SMTP
  secrets are needed to run `sifeucr migrate`.

```sh
kubectl create secret generic sifeucr-runtime --from-env-file=/secure/runtime.env
kubectl create secret generic sifeucr-migrations --from-env-file=/secure/migrations.env
kubectl apply -f contrib/k8s/migrate.yaml
kubectl wait --for=condition=complete job/sifeucr-migrate --timeout=600s
kubectl apply -k contrib/k8s
kubectl rollout status deployment/sifeucr
```

Use a distinct migration Job name per release and update the wait command to match.
Do not apply the Deployment if migrations fail. The runtime SQL user needs normal
application DML, maintenance DELETEs and SELECT on migration metadata (including
`schema_migration_pending` on MySQL); it does not need DDL. PostgreSQL grants must
cover tables and sequences created by the migration owner, including future ones.
See [database privileges](database-privileges.md) for explicit MySQL grants and
the separation of migration and runtime connections.

`--auto-migrate=false` reads metadata without creating tables or taking migration
locks and requires the **exact** latest embedded migration version. Missing,
older, newer and unfinished MySQL schemas fail startup. A mismatched schema is
never silently repaired. This is a conservative compatibility check, not schema
introspection: manual schema drift still requires operator investigation.

Historical migrations are immutable and some require stopping old writers. This
initial deployment must follow their existing upgrade instructions. The supplied
rolling strategy is suitable for replicas of the same schema version. Future
schema releases need expand/contract migrations **and an explicitly reviewed
compatibility range in the version check** before mixed schema-version binaries
can be supported. Moving migrations into a Job alone does not provide zero-downtime
schema upgrades. MySQL partial DDL needs the recovery procedure in AGENTS.md.

## Health, shutdown and capacity

`GET /livez` returns empty 204 independently of SQL. `GET /readyz` pings SQL with a
one-second timeout and returns empty 204 or 503. These reserved absolute paths
ignore `SIFEUCR_ROOT` and do not reveal errors or use authentication. Exclude them from
public gateway routing when appropriate. Initialization finishes before listening.

SIGTERM withdraws readiness immediately, keeps serving for the drain delay, then
closes listeners and finishes active requests within the shutdown timeout. The
Kubernetes example allows 40 seconds for 5 seconds propagation, 25 seconds HTTP
shutdown and cleanup margin. Measure propagation through your actual gateway;
no universal delay guarantees zero dropped requests. Requests ignoring their
context may still be cut off; committed mutations are never undone by shutdown.
Do not automatically retry non-idempotent POST/PATCH requests at the gateway.

The Service is private. Configure your existing gateway for TLS, application root
paths, trusted proxy addresses and per-client-IP request limits. Identity limits
already span all application replicas. Check whether gateway limits are per
controller instance or shared before relying on a global IP budget.

Budget SQL connections using peak live replicas, including surge and terminating
pods, plus migration/administrative headroom. Tune resources from load measurements.
The example requests distribution across nodes and allows one replica to be
voluntarily disrupted; this does not make SQL highly available. Configure SQL
backups/failover separately. Add autoscaling only after measuring application and
SQL capacity. Logs go to stdout; attach pod identity in the log collector. Successful
HTTP requests currently log at debug level; the gateway can provide access logs.

Expiration maintenance remains per replica, bounded and idempotent. It can cause
extra work/lock contention at large replica counts. Introduce a separate worker
or coordination only when measurements justify it. No singleton election or
external scheduler is required for the current cleanup semantics.

## Secrets and delivery semantics

Run without `--dev` in production. The JWT secret requires at least 32 bytes.
Keep it identical across replicas and keep node clocks synchronized. There is
currently one active signing key; a coordinated key rotation invalidates existing
JWTs. Seamless key rotation is not implemented.

SMTP credential verification runs synchronously with a ten-second timeout and
verified TLS, before creating a local account or issuing tokens. No password is
persisted and no verification email is sent. Stop old application writers before
migration 20261116000 removes OTP challenge tables and the email-change permission.

## Verification

The Go integration suite uses disposable databases when `TEST_POSTGRES_DSN` and
`TEST_MYSQL_DSN` are set. Test users require CREATE/DROP DATABASE permissions.
Run `go test -race ./...`. See `python3 contrib/test-multi-instance.py` for a Docker smoke
check using two independent application processes per backend.

Validation completed on this branch:

- `go test -race ./...` with SQLite and Docker PostgreSQL 18 / MySQL 8.4 passed.
- Source-based Docker image build passed, including generated SQL, templates and docs.
- The independent-process smoke test passed on both external backends, using runtime
  SQL users without DDL privileges and read-only container filesystems. It covers
  cross-replica login, issuance/verification budgets, refresh competition, profile
  visibility, revocation, SIGTERM, SQL outage/recovery, schema mismatch rejection,
  repeat migration execution and unfinished MySQL migration rejection.
- All five Kubernetes resources passed strict kubeconform schema validation.
- CI now builds the image and runs the smoke test after the existing Go checks.

The smoke test uses development credential bypass; it does not
contact a real SMTP provider. No Kubernetes cluster rollout or gateway propagation
measurement was performed. Validate those against your deployment environment.

The implementation plan is in [multi-instance-plan.md](multi-instance-plan.md).
