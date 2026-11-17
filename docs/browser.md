# Browser authentication

The landing links directly to login, which redirects authenticated users to home.
Login uses native HTML POST forms. Their small `onsubmit` handler marks
the submit button busy and disables duplicate clicks; CSP authorizes only that
exact handler with a SHA-256 hash. Native server redirects still drive login and
OTP verification. Ordinary login does not load Datastar.

Home and automatic session restoration load Datastar as an external ES module
in the head. Modules are deferred by default. `templates/datastar.go` pins v1.0.5,
verifies download SHA-256 and generates the static path and SRI integrity value.
The browser verifies SRI too. The module URL includes its content SHA-256.
Successful GET/HEAD responses for that exact public asset use `public, max-age=31536000, immutable`, avoiding module
network round trips on warm restoration. Login/home and errors do not inherit
this policy. Changed bytes produce a new URL; SRI still verifies the content.
The binary embeds the file and license; no runtime
CDN or custom inline script bundle is involved. Each reactive document gets a
fresh nonce for Datastar CSP mode; arbitrary inline scripts and eval stay blocked.

`templates/interactions.go` shares form submission, refresh and control attributes.
Buttons start disabled and enhanced links inert until Datastar initializes their
bindings. A pending action ID limits `aria-busy` to its initiating control, while a shared
guard still prevents overlapping actions. The refresh SVG hides while Semantic
displays its spinner. Async actions enforce
same-origin requests and use a 30-second timeout. Errors clear busy state and
allow retry. Local modal open/close actions finish immediately and need no loading
indicator. Cancel and close use native `commandfor` controls.

Forms use named fields with native validation. The shared expression reads JSON
on success and error HTTP statuses, displaying messages as text. This small fetch
expression is necessary because Datastar's built-in backend action ignores error
response bodies. No mutation is automatically retried. POST requests hold an
origin-wide Web Lock, when available, until response cookies are applied.

The existing CSRF-protected `POST /login` adapter calls the same JSON handlers as
the API, retaining validation, quotas, account ownership and live permissions.
Email change requires `perm.change_email`; the code goes to the new address and
confirmation updates the email shown in the heading. Wrong codes retain the form.
API endpoints still authenticate exclusively with Bearer headers.

Refresh/pagination use Datastar GET and morph only the sessions card returned by
`GET /home?fragment=sessions`. Profile, email inputs and document layout survive.
Sessions contain up to 100 rows per cursor page. Device labels use browser and OS
names, such as `Chrome (Linux)`, and an info tag marks Current. The confirmation
uses the same label or “Log out of this device”. Unrecognized metadata falls back
to escaped text and never grants authorization.

Dates have UTC server fallbacks. A Datastar initialization expression uses native
`Intl.DateTimeFormat` for the visitor's timezone and request language; punctuation
and abbreviations follow the browser locale implementation.

A valid access cookie redirects login to home. Otherwise, a refresh cookie enables
automatic POST restoration using Datastar `data-init` and the shared submit
expression. GET never rotates credentials. Restoration requires Web Locks (HTTPS
or localhost); without them, on failure or with rejected credentials it returns
to ordinary login. Prefetch skips restoration; an activated prerender reloads.
OTP verification takes precedence over restoration. A failed renewal never clears
cookies and cannot erase a newer session. Lost successful refresh responses still
require login because refresh credentials are one-time.

Sign out revokes the stable session before clearing cookies. Another session's
revocation refreshes the card; current-device revocation redirects to login.
Dependency failures preserve cookies. Retained consumed refresh hashes can identify
a rotated successor for revocation, never renewal. Access verification checks the
uncached session record. Cookie coordination is not an authorization mechanism.

Cookies are host-only, HttpOnly, SameSite=Strict and root-scoped; production uses
Secure cookies with `__Secure-` names. Tokens never enter HTML, JS or storage.
Production browser POSTs pass `http.CrossOriginProtection`; configure trusted exact
origins with `SIFEUCR_CSRF_ORIGINS` when necessary and preserve Host at the proxy.
Development explicitly bypasses this protection and permits all CORS origins.
API CORS and browser CSRF remain separate. Proxy throttling must include POST login;
identity quotas remain enforced. `Referrer-Policy: strict-origin` preserves native
HTTP LAN form Origin without disclosing paths/queries in referrers.

With JavaScript unavailable, login still submits normally. A noscript link bypasses
automatic restoration. Dashboard controls depending on Datastar stay disabled when
the module cannot load; this avoids partially functional reactive actions.

`npm ci` installs documentation dependencies. Then `go generate ./...` generates
Redocly docs, SQL code, templates and assets; `go build` compiles them into the
binary. No npm postinstall build, frontend bundler or ESLint is required. For
template-only edits, use `go generate ./templates`. Rebuild/restart Go after generating
assets. Remove browser registration, login route references, browser handlers and
templates to remove the surface; API contracts remain independent.
