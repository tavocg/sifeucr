package auth_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	goauth "github.com/tavocg/go-auth"
	"sifeucr/auth"
	"sifeucr/database"
	"sifeucr/internal/testdb"
)

func authenticator(t *testing.T, db database.Database) *auth.Authenticator {
	t.Helper()
	// Empty JWT secrets remain supported by the application's explicit policy.
	a, err := auth.NewAuthenticator(db, "", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func account(t *testing.T, db database.Database, email string) *auth.Claims {
	t.Helper()
	sub, err := db.Querier().EnsureAccountByEmail(t.Context(), email, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	return &auth.Claims{Sub: strconv.FormatInt(sub, 10)}
}

func TestRevokeAllInvalidatesOnlyTheAccountSessions(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		a := authenticator(t, db)
		identity := account(t, db, "owner@example.com")
		at, rt, err := a.Issue(t.Context(), identity)
		if err != nil {
			t.Fatal(err)
		}
		other, _, err := a.Issue(t.Context(), account(t, db, "other@example.com"))
		if err != nil {
			t.Fatal(err)
		}
		if err := a.RevokeAll(t.Context(), identity); err != nil {
			t.Fatal(err)
		}
		if _, err := a.Verify(t.Context(), at.Value); !errors.Is(err, goauth.ErrInvalidToken) {
			t.Fatalf("revoked access token: %v", err)
		}
		if _, _, err := a.Refresh(t.Context(), rt.Value); !errors.Is(err, goauth.ErrInvalidToken) {
			t.Fatalf("revoked refresh token: %v", err)
		}
		if _, err := a.Verify(t.Context(), other.Value); err != nil {
			t.Fatalf("unrelated session was revoked: %v", err)
		}
		fresh, _, err := a.Issue(t.Context(), identity)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Verify(t.Context(), fresh.Value); err != nil {
			t.Fatalf("fresh login after revocation: %v", err)
		}
	})
}

func TestConcurrentRefreshHasOneWinner(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		a := authenticator(t, db)
		_, rt, err := a.Issue(t.Context(), account(t, db, "owner@example.com"))
		if err != nil {
			t.Fatal(err)
		}
		start, results := make(chan struct{}), make(chan error, 12)
		for range 12 {
			go func() {
				<-start
				_, _, err := a.Refresh(t.Context(), rt.Value)
				results <- err
			}()
		}
		close(start)
		winners := 0
		for range 12 {
			if err := <-results; err == nil {
				winners++
			} else if !errors.Is(err, goauth.ErrInvalidToken) {
				t.Errorf("unexpected refresh failure: %v", err)
			}
		}
		if winners != 1 {
			t.Fatalf("successful exchanges = %d, want 1", winners)
		}
	})
}

type failingDatabase struct{ database.Database }
type failingQuerier struct{ database.Querier }

var errDelete = errors.New("injected refresh deletion failure")

func (d failingDatabase) WithTx(ctx context.Context, fn func(database.Querier) error) error {
	return d.Database.WithTx(ctx, func(q database.Querier) error { return fn(failingQuerier{q}) })
}

func (failingQuerier) DeleteRefreshTokensBySub(context.Context, int64) error { return errDelete }

func TestFailedRevokeAllRollsBackGeneration(t *testing.T) {
	testdb.Run(t, func(t *testing.T, db database.Database) {
		a := authenticator(t, db)
		identity := account(t, db, "owner@example.com")
		at, rt, err := a.Issue(t.Context(), identity)
		if err != nil {
			t.Fatal(err)
		}
		if err := authenticator(t, failingDatabase{db}).RevokeAll(t.Context(), identity); !errors.Is(err, errDelete) {
			t.Fatalf("revocation failure = %v", err)
		}
		if _, err := a.Verify(t.Context(), at.Value); err != nil {
			t.Fatalf("generation was not rolled back: %v", err)
		}
		if _, _, err := a.Refresh(t.Context(), rt.Value); err != nil {
			t.Fatalf("refresh token was not preserved: %v", err)
		}
	})
}
