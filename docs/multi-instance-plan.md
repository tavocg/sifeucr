# Multi-instance implementation plan

Target: independent HTTP replicas sharing PostgreSQL or MySQL, without session
affinity, with a repeatable migration/deployment lifecycle. Keep backend locking
and SQL ownership explicit; use the existing small interfaces and standard library.

1. **Database lifecycle and configuration**: separate connection from migration,
   add `sifeucr migrate` and schema validation for non-migrating servers, bound pool
   sizes, and make local profile caching optional (off by default).
2. **HTTP lifecycle**: independent liveness/readiness checks, bounded SQL health
   checks, readiness withdrawal before draining, and shutdown budgets compatible
   with existing request deadlines. Keep operational routes outside application
   routing and localization.
3. **Deployment**: build an OCI image, provide Kubernetes Deployment/Service and
   a separate migration Job, configure shared secrets and resources, and document
   schema compatibility, key rotation, gateway limits and connection budgeting.
   Wire the existing SMTP provider so OTP delivery can be tested end to end.
4. **Verification**: run existing tests with Docker PostgreSQL/MySQL and race
   detection; exercise independent replicas, OTP budgets, refresh competition,
   revocation, profile freshness, probes and termination. Commit reusable smoke
   tests and record results.

Scope decisions: SQL remains authoritative. No Redis, message broker, leader
election, new auth protocol or Kubernetes SDK. Maintenance remains bounded and
idempotent per replica. Secrets stay identical across replicas; rotating them
currently requires a coordinated cutover and invalidates existing credentials.
Historical migrations remain immutable; incompatible upgrades require downtime.
Future schema changes must use expand/contract for rolling upgrades. Metrics and
autoscaling require workload-specific measurements and are not prerequisites for
running multiple replicas.

References consulted (patterns, not dependencies):
- [Kubernetes termination](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination)
- [controller-runtime health handlers](https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/healthz/healthz.go)

All four stages are implemented. Validation results and operational limits are
recorded in [the deployment guide](multi-instance.md#verification).
