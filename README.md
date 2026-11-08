# SIFEUCR

## Usage

```sh
npm ci
go generate ./...

# Start a development server
go run . --dev
# { "msg":"listening","addr":"[::]:3080" }
```

## Extending

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
  permissions, OTPs or refresh tokens.
- Add messages to both locales, update `docs/api.yaml`, and run `npm run redocly`.

See [AGENTS.md](AGENTS.md) for project conventions and verification commands.
