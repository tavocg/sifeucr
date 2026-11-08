package middleware

import (
	"net/http"

	"sifeucr/internal/httpx"
)

type LocalizeFunc func(r *http.Request, key string, args ...any) string

func writeJSONError(w http.ResponseWriter, r *http.Request, localize LocalizeFunc, status int, key string) {
	status, key = httpx.ErrorStatus(r.Context(), status, key)
	msg := key
	if localize != nil {
		msg = localize(r, key)
	}

	httpx.WriteJSONError(w, status, msg)
}
