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
