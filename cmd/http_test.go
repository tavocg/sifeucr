package cmd

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sifeucr/internal/health"
)

func TestShutdownWithdrawsReadinessAndFinishesRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	h := health.New(ctx, func(context.Context) error { return nil }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(204)
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h}
	defer srv.Close()
	done := make(chan error, 1)
	go func() { done <- serveHTTP(ctx, srv, listener, 100*time.Millisecond, time.Second) }()
	client := &http.Client{Timeout: 2 * time.Second}
	result := make(chan int, 1)
	go func() {
		resp, err := client.Get("http://" + listener.Addr().String() + "/work")
		if err != nil {
			result <- 0
			return
		}
		resp.Body.Close()
		result <- resp.StatusCode
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	// Query the same handler directly so routing propagation timing cannot make
	// this assertion depend on a scheduler beating the listener's shutdown.
	req, _ := http.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal("readiness remained available")
	}
	release <- struct{}{}
	if status := <-result; status != 204 {
		t.Fatalf("active request interrupted: %d", status)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
