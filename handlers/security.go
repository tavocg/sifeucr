package handlers

import (
	"net/http"

	"sifeucr/templates"
)

// Permit only the exact native form busy handler; arbitrary inline code stays blocked.
var documentCSP = "default-src 'self'; script-src 'self'; script-src-attr 'unsafe-hashes' '" + templates.NativeBusyHash + "'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-src 'self'; frame-ancestors 'self'"

// Apply only to our rendered pages; Redocly has separate resource requirements.
func documentHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", documentCSP)
	// Preserve same-origin form Origin on HTTP LAN hosts without exposing queries.
	w.Header().Set("Referrer-Policy", "strict-origin")
}
