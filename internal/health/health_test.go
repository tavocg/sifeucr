package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProbes(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var failure error
	calls := 0
	h := New(ctx, func(ctx context.Context) error {
		calls++
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Error("unbounded readiness check")
		}
		return failure
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(418) }))
	check := func(path string, status int) {
		t.Helper()
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != status {
			t.Fatalf("%s: got %d, want %d", path, w.Code, status)
		}
		if path != "/sifeucr" && (w.Body.Len() != 0 || w.Header().Get("Cache-Control") != "no-store") {
			t.Fatal("probe leaks details or is cacheable")
		}
	}
	check("/sifeucr", 418)
	check("/readyz", 204)
	failure = errors.New("private SQL error")
	check("/readyz", 503)
	check("/livez", 204)
	if calls != 2 {
		t.Fatal("liveness queried SQL")
	}
	failure = nil
	check("/readyz", 204)
	cancel()
	check("/readyz", 503)
	check("/livez", 204)
	if calls != 3 {
		t.Fatal("draining probe queried SQL")
	}
}
