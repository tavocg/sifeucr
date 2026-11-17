# SIFEUCR

Based on [go-kickstart](https://github.com/alturalabscr/go-kickstart),
updated through commit `ea9fe4d973de3cb4d2c5e8f38e69bf11941beb9f`.

Copyright (C) 2026 Gustavo Calvo <tavo@tavo.cr>.
Licensed under [GPL-3.0-or-later](LICENSE).

## Usage

Requirements: Go 1.26.11 or newer (see `go.mod`), Node.js 22.12 or newer and
npm 10 or newer. Run these commands from the repository root:

```sh
npm ci               # install the exact documentation dependencies from the lockfile
go generate ./...   # generate API docs, SQL code, templates and pinned assets
go run . --dev      # start the development server
```

For a compiled binary, replace the last command with:

```sh
go build -o sifeucr .
```

Run `npm ci` on a fresh checkout or when `package-lock.json` changes. After that,
`go generate ./...` followed by `go build -o sifeucr .` is sufficient. Generation
requires network access to download Go tools and pinned assets; Node/npm are
build-time dependencies and are not needed to run the resulting binary.
Generated files are ignored, so a fresh checkout must generate before building.
Go downloads its module/tool dependencies automatically; it does not install npm
dependencies or run generators as part of `go build`.

For focused edits, use `go generate ./docs`, `go generate ./database` or
`go generate ./templates`. CI and Docker follow the same install → generate →
build sequence, without an npm installation hook that builds files implicitly.

For automatic rebuilds after the initial generation, run:

```sh
go tool air
```

Air is pinned in `go.mod` and runs the sifeucr with `--dev`. `.air.toml` watches Go,
templ, SQL, API docs and runtime configuration. It regenerates only the affected
outputs before rebuilding, and ignores its temporary binary and database files.
Install npm dependencies and run `go generate ./...` once before starting it.
Extra application flags can be passed with `go tool air -- --port 3081`.

Browser login, protected home and session listing are described in
[the browser authentication guide](docs/browser.md).

`--dev` defaults to debug-level text logs with source locations, skips institutional credential verification,
permits all CORS origins and bypasses browser form CSRF checks. It also allows
empty/short signing secrets and uses browser cookies suitable for
local HTTP. Development skips the readiness drain delay on shutdown while
still allowing active requests to finish. Production keeps the configured
`http.drain-delay`. Explicit
log settings in flags, environment or configuration override the dev defaults.

Production login requires `auth.smtp.address` (or `SIFEUCR_AUTH_SMTP_ADDRESS`)
with the institutional SMTP host and port. Implicit TLS is the default; set
`auth.smtp.starttls=true` for required STARTTLS. Users sign in with their email
and institutional password. Development accepts a valid email without verification.
Stop old writers before applying migration 20261116000, which removes the retired
OTP challenge tables and email-change permission while preserving accounts and sessions.

## Configuration

Copy [`contrib/sifeucr.example.yaml`](contrib/sifeucr.example.yaml) to `sifeucr.yaml` in the
working directory to customize the defaults, or pass `--config /path/to/sifeucr.yaml`.
Flags override environment variables, which override the configuration file.
For environment variables, replace dots and hyphens in keys with underscores
and prefix with `SIFEUCR_` (for example, `SIFEUCR_AUTH_ACCESS_TOKEN_TTL=15m`).

Reverse proxy examples for Nginx, Caddy and Apache are in [the reverse proxy guide](docs/reverse-proxy.md).
The default Dockerfile runs Go behind nginx on port 80; see
[Railway behind Cloudflare](docs/reverse-proxy.md#railway-behind-cloudflare).

Session listing and individual or account-wide logout are available through the JSON API; see [session management](docs/sessions.md), including deployment notes.

See [multiple instances and Kubernetes](docs/multi-instance.md) for container builds,
separate migration jobs, probes, graceful shutdown and shared SQL configuration.

## Extending

See [named routes and CORS](docs/routes.md) for shared URL helpers, startup
validation and handler/endpoint origin policies.
See [build security](docs/build-security.md) for CI audits, pinned build inputs
and the weekly dependency review.

- `go generate ./templates` downloads [Semantic CSS](https://github.com/tavocg/semantic)
  from the release tag pinned in `templates/semantic.go` and writes its minified CSS to
  `handlers/static/css/semantic.<sha256>.css` with the upstream licenses. It also
  generates `templates.SemanticStylesheet()`, rendered inline in the shared page
  head once per document through templ's OnceHandle. Dashboard fragment updates
  preserve that head; each new HTML document includes the CSS again. The static CSS and
  `templates.SemanticStylesheetURL(links)` remain available to other consumers.
  It verifies the CSS against `semanticSHA256` before writing files and
  limits each CSS/license download to 1 MiB. Review the new release, compute the
  downloaded CSS digest with `sha256sum`, then update both `semanticTag` and
  `semanticSHA256` and regenerate to upgrade; changed CSS gets a new
  URL automatically, and obsolete generated stylesheets are removed. Generation
  requires network access; the built server embeds and serves the CSS locally.
- `go generate ./templates` also downloads the Datastar release pinned in
  `templates/datastar.go`, verifies its SHA-256 and generates its static path and
  SRI integrity value. The head loads the external module only on reactive pages.
  Browser interactions live in templ attributes and `templates/interactions.go`;
  no frontend bundler, TypeScript or ESLint is required. `npm ci` installs the
  documentation tool; `go generate ./...` runs all generators. See [browser authentication](docs/browser.md).
- Add a route group in `handlers/`, a registration method and its call in `registerRoutes`.
- Use `Add` for a handler function and `AddHandler` for middleware-wrapped handlers;
  both apply the configured root prefix.
- For endpoints with multiple response formats, see [content negotiation](docs/content-negotiation.md); JSON is the default.
- Keep request/response types near their handler. Use `decodeBody` for localized
  validation errors and `internal/httpx` for JSON responses.
- Put reusable behavior in `middleware/`; authorization checks current permissions
  with one uncached database query.
- Add matching migrations and queries for SQLite, PostgreSQL and MySQL, then run
  `go generate ./database`. Keep driver-specific details in their adapters.
- Keep read-through caching in `database/cached`; never cache authentication,
  permissions or refresh tokens.
- Add messages to both locales, update `docs/api.yaml`, and run `npm run redocly`.

See [AGENTS.md](AGENTS.md) for project conventions and verification commands.
