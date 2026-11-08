// Package logging defines the minimal logger used by handlers and middleware.
package logging

import "net/http"

type Logger interface {
	Debug(msg string, args ...any)
	Error(msg string, args ...any)
}

// RequestArgs supplies consistent request context without logging bodies or credentials.
func RequestArgs(r *http.Request, status int, extra ...any) []any {
	args := make([]any, 0, 6+len(extra))
	args = append(args, "status", status, "method", r.Method, "path", r.URL.Path)
	return append(args, extra...)
}
