# Named routes and CORS

`routes` is the shared catalogue of application methods and paths. Register a
catalogue entry with `h.AddRoute(routes.Account, handler)`. Construct links with
`h.URL(routes.Account)` in handlers or `links.URL(routes.APIDocs)`
in templ. Create `links` with `routes.NewLinks(rootPrefix)` once; it validates
and stores the prefix in an immutable value. The handler uses this same instance
for registration and passes it in `base.PageParams.Links` to the page, including
the 404 page. `links.StaticURL(name)` also prefixes and escapes embedded asset
paths. No global prefix or per-call prefix argument is used. Both URL functions return `(string, error)`, which templ handles.
Parameterized routes require values: `h.URL(routes.RevokeSession, sessionID)`.
Values are escaped as individual path segments; missing or extra values fail.

The compiler checks symbol names and Go types. `NewHandler` additionally checks
that every catalogue entry was registered and that the linked API documentation,
generated stylesheet and JavaScript bundle exist in the embedded filesystem. Renaming a route
breaks references at compilation; forgetting registration fails construction.
This does not statically inspect arbitrary URL strings. Use the named helpers
for internal links. Dynamic identifiers are validated when constructing a URL;
the router cannot establish that a referenced database record exists.

`handlers` imports templates, so templates import the independent `routes`
package to avoid an import cycle. There is no route code generator, network
lookup or filesystem access per request. `Add` and `AddHandler` remain available
for extensions using literal paths; these are outside catalogue completeness
checks. Finish all registration before serving concurrent requests.

`Add`, `AddHandler` and `AddRoute` provide automatic `OPTIONS` responses. Plain
OPTIONS returns 204 with `Allow`. Preflight checks the requested method against
the router, without invoking authentication or endpoint code. Unknown paths
retain the normal 404 response. Explicit OPTIONS or methodless handlers own
their OPTIONS behavior and take precedence over automatic handling.

CORS is disabled by default outside development. With `--dev` / `Options.Dev`,
all origins are permitted, including on endpoints with an explicit origin policy;
method and request-header checks still apply. Set `handlers.Options.CORSOrigins`,
or configure
`cors.origins` / `--cors-origins` / `SIFEUCR_CORS_ORIGINS`. YAML accepts a list; flags
and environment variables accept comma-separated exact HTTP(S) origins, e.g.
`https://web.example.com` (without a path or trailing slash).

Endpoint options replace the inherited list:

```go
h.AddRoute(routes.Account, handler, handlers.RouteOptions{
    CORSOrigins: []string{"https://admin.example.com"},
})
```

The same optional argument works on `Add` and `AddHandler`. Outside development,
nil inherits; an explicit empty slice disables CORS for that endpoint. GET also permits HEAD.
Preflights allow `Authorization`, `Content-Type`, `Accept` and `Accept-Language`.
Disallowed origins, methods or headers return localized 403 without CORS grant
headers. Responses from permitted actual requests, including authentication
errors, carry `Access-Control-Allow-Origin` and `Vary: Origin`. Browser cookies
and other credentialed CORS requests are not enabled; Bearer Authorization
headers are supported. CORS controls browser response access, not authorization
or whether a simple cross-origin request can execute.

Operational probes currently use their separate health router and are not
registered through Handler.Add; these policies apply to application routes.
