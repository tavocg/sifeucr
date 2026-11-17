# Institutional browser access

GET /login displays a native email/password form. POST /login adapts POST
/auth/login, verifies institutional credentials and stores session tokens in
HttpOnly, SameSite=Strict cookies (Secure outside development). Passwords never
enter cookies, redirect URLs, database records or logs. Development bypasses
credential verification.

Configure auth.smtp.address (host:port) and optionally auth.smtp.starttls for
required STARTTLS; otherwise implicit TLS is used. Certificates are verified.
The application permits five attempts per ten minutes per normalized email.
Apply additional per-client-IP throttling at the reverse proxy.

GET /dashboard and GET /dashboard/account require a valid browser session. The Semantic
aside links Overview and Account; Account shows the email, account creation date,
and paginated active sessions with device details and revocation controls.
The avatar is an inline SVG using the uppercase email initial and a stable color
derived from that letter.
Overview uses the MIT-licensed Heroicons squares-2x2 icon; see licenses/heroicons.txt.

Account session refresh and revocation use pinned embedded Datastar with SRI and
CSP nonce mode. Login uses native forms and the CSP-hashed busy handler.
Cross-origin POST requests are rejected outside development unless an origin
is explicitly trusted. API endpoints accept Bearer tokens only.

GET never rotates refresh tokens. Login restoration uses a CSRF-protected POST
coordinated with Web Locks between tabs. Prefetch and prerender requests never
consume a rotating token. Failed renewal preserves cookies; explicit logout
revokes the session before clearing them. Without JavaScript, login and session
revocation remain available; automatic restoration and modal controls require JS.

Email changes and OTP login are unavailable. Deploy with old writers stopped:
migration 20261116000 removes their challenge tables and email-change permission.
Existing sessions and account addresses are preserved.

The shared header displays the application name and links Home to the public root
and Dashboard to /dashboard. Dashboard is marked current throughout its pages, including
Account. The shared footer links /about,
/terms, the API documentation, the generated /s/LICENSE.txt, and the source
repository. Information pages are public, localized and use no-store.
All application links include the configured root prefix.
