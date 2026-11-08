# Response formats

The root page and router-generated 404 responses offer JSON and HTML. They use
`Accept` quality weights and wildcards to choose a supported format. JSON wins
when their weights are equal and is also the fallback when no supported format
matches. Unsupported preferences return JSON instead of a 406 error.

| Accept | Response format |
| --- | --- |
| Missing, empty or `*/*` | JSON |
| `text/html` | HTML |
| `application/json` | JSON |
| `application/xml` or another unsupported type | JSON |
| `text/html;q=0, */*` | JSON |
| `text/html;q=0.9, application/json;q=0.5` | HTML |
| `text/html;q=0.5, application/json;q=0.9` | JSON |

An explicit `q=0` excludes that representation from negotiation. If every
supported representation is excluded, the server still uses its JSON fallback.
Router errors preserve their status, including 404. Account and authentication
endpoints currently return JSON; their response format does not change when
`Accept` requests HTML. Static assets retain their own media types.

`Content-Type` describes the request body; it does not select the response.
JSON endpoints accept requests with no Content-Type or with another value,
including `text/plain`. Their bodies must still contain one valid JSON value,
with no unknown fields and a maximum size of 1 MiB. URL-encoded and multipart
forms are not decoded as JSON.

## Adding a format to an endpoint

Use `internal/httpx.Negotiate` with the media types the endpoint can render,
putting its default first. For example, an application extending this template
could add XML to the same route:

```go
format := httpx.Negotiate(r,
    "application/json; charset=utf-8",
    "text/html; charset=utf-8",
    "application/xml; charset=utf-8",
)
```

Add a renderer branch for each additional type. For XML, set `Content-Type` to
the selected format and encode the response with `encoding/xml`. Keep the
existing HTML renderer and JSON fallback. Merely adding a type to the list does
not implement its encoder. The template itself only offers JSON and HTML.

Set `Vary: Accept` before writing headers; include `Accept-Language` when the
content is localized. The template's negotiated responses also send
`Cache-Control: no-store`. Keep those headers on every representation and error
produced by that endpoint. Update `docs/api.yaml` to list each response media
type the endpoint actually supports.
