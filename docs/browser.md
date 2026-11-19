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

GET /my requires a valid browser session and renders Home, Account,
account details and the first sessions page in one document. The Semantic aside
links #home and #account. Pinned Datastar reacts to hash changes to switch
panels, update the selected link and page title, and close the mobile sidebar.
Switching panels does not fetch a new document. Direct links, reloads and browser
Back/Forward restore the selected panel. Without JavaScript, both panels remain
visible and the links act as native section anchors.

GET /my/account is retained as a no-store redirect to /my#account,
preserving a supplied sessions cursor. Session pagination, refresh and revocation
continue to use the existing authenticated handlers and partial session updates.
Pagination's full-page fallback returns to #account.
The avatar is an inline SVG using the uppercase email initial and a stable color
derived from that letter.
Home uses the MIT-licensed Heroicons home icon; see licenses/heroicons.txt.

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

The shared header links the application name to the public root. Its navigation
is hidden inside the personal area; public pages show a single Log in link to /login.
The shared footer links /about,
/terms, the API documentation, the generated /s/LICENSE.txt, and the source
repository. Information pages are public, localized and use no-store.
All application links include the configured root prefix.
