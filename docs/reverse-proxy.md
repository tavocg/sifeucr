# Reverse proxy examples

Choose one proxy and replace `example.com` with your domain. These examples
assume the proxy and application run on the same host. Start the application
with `--host 127.0.0.1 --port 3080` and configure its secrets as usual.

| Proxy | Example | Setup / validation |
| --- | --- | --- |
| Nginx | [`contrib/nginx.example.conf`](../contrib/nginx.example.conf) | Include inside `http`; install your certificate/key at the shown paths; run `nginx -t`. |
| Caddy | [`contrib/Caddyfile.example`](../contrib/Caddyfile.example) | Copy to `/etc/caddy/Caddyfile`; run `caddy validate --config /etc/caddy/Caddyfile`. Caddy manages HTTPS for a domain pointing to this server with ports 80/443 reachable. |
| Apache 2.4 | [`contrib/apache.example.conf`](../contrib/apache.example.conf) | Enable `ssl`, `proxy`, `proxy_http`, `headers`, `filter`, `deflate`, `alias` and listening on 80/443. Install the certificate/key; include the virtual hosts; run `apachectl configtest`. On Debian/Ubuntu, use `a2enmod` to enable modules. |

All three preserve the request path and negotiate gzip for text assets under
`/s/`. API responses are outside that compression scope. If using `--root /sifeucr`,
prepend `/sifeucr` to the static matcher and Nginx's rate-limited paths; keep the
proxy upstream mapping unchanged so the application receives the prefix.

Nginx includes per-IP limits: issuance 10/minute, verification 30/minute and
refresh 60/minute, with small bursts and `429` rejections. These are tunable
average rates (leaky buckets), not fixed-window quotas. Login and email changes
share their issuance/verification zones. The application's identity limits also
apply.

The stock Caddy and Apache examples do **not** enforce per-IP request limits.
Caddy's [rate-limit handlers](https://caddyserver.com/docs/modules/http.handlers.rate_limit)
require an extra module. Apache's
[`mod_ratelimit`](https://httpd.apache.org/docs/2.4/mod/mod_ratelimit.html)
limits response bandwidth, not request counts. Add request throttling through
an appropriate extension or an upstream gateway when using those examples.

These configurations assume direct client connections to the proxy and replace
forwarded client information rather than accepting arbitrary forwarding headers.
If placing a CDN or another proxy in front, configure trusted proxies and the
client-IP rate-limit key for that deployment before relying on per-IP limits.
For containers on different hosts/networks, replace the loopback upstream with
the application's private address and prevent public access to its port.
