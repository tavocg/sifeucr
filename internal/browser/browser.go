// Package browser adapts existing JSON handlers to server-rendered browser flows.
// It does not change authentication or teach API endpoints to accept cookies.
package browser

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type Reply struct {
	Status    int
	HeaderMap http.Header
	Body      bytes.Buffer
}

func (r *Reply) Header() http.Header { return r.HeaderMap }
func (r *Reply) WriteHeader(status int) {
	if r.Status == 0 {
		r.Status = status
	}
}
func (r *Reply) Write(b []byte) (int, error) { r.WriteHeader(200); return r.Body.Write(b) }

// Call invokes the same handler, retaining context, headers and device metadata.
func Call(r *http.Request, next http.Handler, payload any, bearer string) *Reply {
	request := r.Clone(r.Context())
	if payload != nil {
		body, _ := json.Marshal(payload)
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Del("Authorization")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	reply := &Reply{HeaderMap: make(http.Header)}
	next.ServeHTTP(reply, request)
	reply.WriteHeader(200)
	return reply
}

type Tokens struct {
	Access     string `json:"access_token"`
	Refresh    string `json:"refresh_token"`
	AccessTTL  int    `json:"expires_in"`
	RefreshTTL int    `json:"refresh_token_expires_in"`
}

type Cookies struct {
	Path   string
	Secure bool
}

func (c Cookies) name(kind string) string {
	if c.Secure {
		return "__Secure-sifeucr_" + kind
	}
	return "sifeucr_" + kind
}
func (c Cookies) Read(r *http.Request, kind string) string {
	cookie, err := r.Cookie(c.name(kind))
	if err != nil {
		return ""
	}
	return cookie.Value
}
func (c Cookies) set(w http.ResponseWriter, kind, value string, ttl int) {
	http.SetCookie(w, &http.Cookie{Name: c.name(kind), Value: value, Path: c.Path,
		HttpOnly: true, Secure: c.Secure, SameSite: http.SameSiteStrictMode,
		MaxAge: ttl, Expires: time.Now().Add(time.Duration(ttl) * time.Second)})
}
func (c Cookies) Save(w http.ResponseWriter, tokens Tokens) {
	c.set(w, "access", tokens.Access, max(1, tokens.AccessTTL))
	c.set(w, "refresh", tokens.Refresh, max(1, tokens.RefreshTTL))
}
func (c Cookies) Clear(w http.ResponseWriter) {
	c.set(w, "access", "", -1)
	c.set(w, "refresh", "", -1)
}
