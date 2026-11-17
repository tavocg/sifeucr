# Session management

All routes require `Authorization: Bearer <access_token>` and use the configured
root prefix. They are JSON API endpoints; an application can provide its own UI.

- `GET /account/sessions` lists up to 100 open sessions, newest first. Pass the
  returned `next_cursor` as `?cursor=...` until it is absent.
- `DELETE /account/sessions/{id}` closes one session belonging to the caller.
  Missing sessions and identifiers owned by another account both return `204`.
- `DELETE /account/sessions` closes all the caller's sessions without deleting
  the account. Both deletion endpoints take no body and return an empty `204`.

Each list entry contains `id`, `user_agent`, `created_at`, `last_used_at`,
`expires_at` and `current`. Times are Unix seconds in UTC. `last_used_at` means
last token issuance or refresh, not last API activity. `expires_at` is when both
access and refresh tokens expire; refreshing may extend it. A zero expiry is
possible for legacy refresh tokens without expiration.

The User-Agent is captured at login, limited to 512 UTF-8 bytes, stripped of
control characters and preserved across refreshes. It is client-supplied text,
not a verified device name or fingerprint; display it as plain text. Missing
User-Agents and sessions migrated from older versions have an empty value. No
IP address or location is inferred from untrusted proxy headers. Session lists
never expose access tokens, refresh tokens or their hashes, and responses use
`Cache-Control: no-store`.

Closing a session rejects its access and refresh tokens on subsequent requests.
Other sessions remain valid when closing only one. Closing all sessions also
rotates the account generation. Requests authenticated before revocation may
finish. After closing the current session, discard its credentials locally;
a fresh login can create another session.

Refresh-based revocation resolves the session before acquiring the account lock,
then deletes by its stable identifier in that transaction. It therefore also
revokes a replacement refresh issued between lookup and locking. Consumed hashes
also identify that session until the original refresh/access credentials expire.
For legacy non-expiring refreshes, retention uses one configured refresh TTL from
consumption or the original access expiry, whichever is later. Unknown or expired
references remain an idempotent no-op.

Rotation writes the consumed hash and its account/session generation in the same
transaction as consumption and replacement. It stores no plaintext credentials.
Retained hashes only permit revocation: reusing one to refresh still returns 401
and does not automatically terminate the successor. This avoids treating a
legitimate duplicate request as grounds to close the session; it also means replay
does not automatically contain an attacker who already obtained the successor.
Recovery after a lost successful refresh response still requires login.

Maintenance drains expired references in batches of 100 under its existing time
budget. Account deletion cascades to these references. Each successful rotation
adds one row, retained until its original expiry even if the session closes sooner;
size therefore depends on rotation frequency and credential TTLs. Monitor this
table alongside other expired-record backlogs.

## Deploying migration 20261114000

Stop old application writers before upgrading. The additive migration creates
`consumed_refresh_tokens` without changing existing sessions or credentials. Hashes
consumed by an old binary before deployment cannot be recovered; retention covers
rotations made by the upgraded application. Run `sifeucr migrate` with migration
credentials before starting replicas with auto-migration disabled. MySQL's pending
migration recovery procedure still applies.

## Deploying migration 20261108000

Stop old application versions before starting the upgraded application. The
migration adds session identifiers and device metadata on every database
backend. Existing refresh tokens, account generations and creation times are
preserved, and device information starts as unknown.

Previously issued access tokens lack `sid` and are rejected after the upgrade.
Clients can use their existing refresh token at `POST /auth/refresh` to obtain
new tokens linked to their preserved session. A subsequent fresh login records
the device's User-Agent. Do not run old token writers against the new schema.
For unfinished MySQL migrations, follow the recovery procedure in [AGENTS.md](../AGENTS.md#mysql-migration-recovery).
