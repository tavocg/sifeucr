package cached_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"sifeucr/cache"
	"sifeucr/cache/memory"
	"sifeucr/database"
	"sifeucr/database/cached"
	"sifeucr/internal/testdb"
)

func account(t *testing.T, db database.Database) int64 {
	t.Helper()
	sub, err := db.Querier().EnsureAccountByEmail(t.Context(), "old@example.com", time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	return sub
}

func store(t *testing.T) *memory.Store {
	t.Helper()
	s := memory.New()
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func profile(t *testing.T, q database.Querier, sub int64, email string) {
	t.Helper()
	value, err := q.SelectAccountBySub(t.Context(), sub)
	if err != nil || value == nil || value.Email != email {
		t.Fatalf("profile = %v, %v; want email %q", value, err, email)
	}
}

type pausedDatabase struct {
	database.Database
	loaded, resume chan struct{}
	once           sync.Once
}

func (d *pausedDatabase) Querier() database.Querier {
	return pausedQueries{d.Database.Querier(), d}
}

type pausedQueries struct {
	database.Querier
	db *pausedDatabase
}

func (q pausedQueries) SelectAccountBySub(ctx context.Context, sub int64) (*database.Account, error) {
	value, err := q.Querier.SelectAccountBySub(ctx, sub)
	q.db.once.Do(func() {
		close(q.db.loaded)
		select {
		case <-q.db.resume:
		case <-ctx.Done():
		}
	})
	return value, err
}

func TestOldReadCannotRefillAfterInvalidation(t *testing.T) {
	testdb.RunUncached(t, func(t *testing.T, base database.Database) {
		sub := account(t, base)
		paused := &pausedDatabase{Database: base, loaded: make(chan struct{}), resume: make(chan struct{})}
		release := sync.OnceFunc(func() { close(paused.resume) })
		t.Cleanup(release)
		db := cached.New(paused, store(t))
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		type result struct {
			value *database.Account
			err   error
		}
		read := make(chan result, 1)
		go func() { value, err := db.Querier().SelectAccountBySub(ctx, sub); read <- result{value, err} }()
		select {
		case <-paused.loaded:
		case <-ctx.Done():
			t.Fatal("initial read did not reach the database")
		}
		updated := make(chan error, 1)
		go func() { updated <- db.Querier().DeleteAccount(ctx, sub) }()
		select {
		case err := <-updated:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal("in-flight read prevented the update from completing")
		}
		release()
		select {
		case old := <-read:
			if old.err != nil || old.value == nil || old.value.Email != "old@example.com" {
				t.Fatalf("initial snapshot = %v, %v", old.value, old.err)
			}
		case <-ctx.Done():
			t.Fatal("initial read did not finish")
		}
		absent(t, db, db.Querier(), sub)
		absent(t, db, db.Querier(), sub)
	})
}

var errCache = errors.New("injected cache failure")

type unreliableStore struct {
	cache.Store
	failGet, failSet, failDelete, corrupt bool
	sets, deletes                         atomic.Int64
}

func (s *unreliableStore) Get(ctx context.Context, key string) ([]byte, error) {
	if s.failGet {
		return nil, errCache
	}
	if s.corrupt {
		return []byte("invalid JSON"), nil
	}
	return s.Store.Get(ctx, key)
}

func (s *unreliableStore) Set(ctx context.Context, key string, value []byte) error {
	s.sets.Add(1)
	if s.failSet {
		return errCache
	}
	return s.Store.Set(ctx, key, value)
}

func (s *unreliableStore) Delete(ctx context.Context, key string) error {
	s.deletes.Add(1)
	if s.failDelete {
		return errCache
	}
	return s.Store.Delete(ctx, key)
}

func TestCacheFailuresPreserveDatabaseBehavior(t *testing.T) {
	for _, failure := range []string{"get", "set", "delete", "corrupt"} {
		t.Run(failure, func(t *testing.T) {
			testdb.RunUncached(t, func(t *testing.T, base database.Database) {
				sub := account(t, base)
				s := &unreliableStore{
					Store: store(t), failGet: failure == "get", failSet: failure == "set",
					failDelete: failure == "delete", corrupt: failure == "corrupt",
				}
				db := cached.New(base, s)
				profile(t, db.Querier(), sub, "old@example.com")
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				if _, err := db.Querier().SelectAccountBySub(ctx, sub); !errors.Is(err, context.Canceled) {
					t.Fatalf("cache hid cancellation: %v", err)
				}
				if err := db.Querier().DeleteAccount(t.Context(), sub); err != nil {
					t.Fatal(err)
				}
				if _, err := db.Querier().SelectAccountBySub(t.Context(), sub); !db.IsErrNotFound(err) {
					t.Fatalf("deleted account still readable: %v", err)
				}
			})
		})
	}
}

func TestTransactionBypassesCacheAndInvalidatesOnlyOnCommit(t *testing.T) {
	testdb.RunUncached(t, func(t *testing.T, base database.Database) {
		sub := account(t, base)
		s := &unreliableStore{Store: store(t)}
		db := cached.New(base, s)
		profile(t, db.Querier(), sub, "old@example.com")
		sets, deletes := s.sets.Load(), s.deletes.Load()
		rollback := errors.New("rollback")
		err := db.WithTx(t.Context(), func(q database.Querier) error {
			if err := q.DeleteAccount(t.Context(), sub); err != nil {
				return err
			}
			absent(t, db, q, sub)
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatal(err)
		}
		profile(t, db.Querier(), sub, "old@example.com")
		if s.sets.Load() != sets || s.deletes.Load() != deletes {
			t.Fatal("rolled-back transaction changed the cached entry")
		}
		if err := db.WithTx(t.Context(), func(q database.Querier) error {
			if err := q.DeleteAccount(t.Context(), sub); err != nil {
				return err
			}
			absent(t, db, q, sub)
			if s.sets.Load() != sets || s.deletes.Load() != deletes {
				t.Error("cache changed before the transaction committed")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if s.deletes.Load() != deletes+1 {
			t.Fatal("commit did not invalidate the profile")
		}
		absent(t, db, db.Querier(), sub)
	})
}

func absent(t *testing.T, db database.Database, q database.Querier, sub int64) {
	t.Helper()
	if _, err := q.SelectAccountBySub(t.Context(), sub); !db.IsErrNotFound(err) {
		t.Fatalf("deleted profile remains readable: %v", err)
	}
}
