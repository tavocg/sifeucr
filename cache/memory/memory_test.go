package memory_test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"sifeucr/cache"
	"sifeucr/cache/memory"
)

func TestCapacityEvictsOldestWrite(t *testing.T) {
	for _, limits := range []struct {
		name           string
		entries, bytes int
	}{{"entries", 2, 100}, {"bytes", 100, 8}} {
		t.Run(limits.name, func(t *testing.T) {
			store := memory.New(memory.WithMaxEntries(limits.entries), memory.WithMaxBytes(limits.bytes))
			defer closeStore(t, store)
			for _, key := range []string{"a", "b"} {
				if err := store.Set(t.Context(), key, []byte("old")); err != nil {
					t.Fatal(err)
				}
			}
			if err := store.Set(t.Context(), "a", make([]byte, limits.bytes+1)); !errors.Is(err, cache.ErrTooLarge) {
				t.Fatalf("oversized replacement: %v", err)
			}
			if value, err := store.Get(t.Context(), "a"); err != nil || string(value) != "old" {
				t.Fatalf("oversized write lost original: %q, %v", value, err)
			}
			if err := store.Set(t.Context(), "c", []byte("new")); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Get(t.Context(), "a"); !errors.Is(err, cache.ErrNotFound) {
				t.Fatalf("reading the oldest entry prevented eviction: %v", err)
			}
			if err := store.Set(t.Context(), "b", []byte("x")); err != nil {
				t.Fatal(err)
			}
			if entries, bytes := store.Stats(); entries != 2 || bytes != 6 {
				t.Fatalf("replacement accounting = %d entries, %d bytes", entries, bytes)
			}
			if err := store.Delete(t.Context(), "b"); err != nil {
				t.Fatal(err)
			}
			if entries, bytes := store.Stats(); entries != 1 || bytes != 4 {
				t.Fatalf("deletion accounting = %d entries, %d bytes", entries, bytes)
			}
		})
	}
}

func TestValuesAreCopiesAndCanceledWritesHaveNoEffect(t *testing.T) {
	store := memory.New()
	defer closeStore(t, store)
	value := []byte("original")
	if err := store.Set(t.Context(), "key", value); err != nil {
		t.Fatal(err)
	}
	value[0] = 'X'
	read, err := store.Get(t.Context(), "key")
	if err != nil || string(read) != "original" {
		t.Fatalf("caller modified retained value: %q, %v", read, err)
	}
	read[0] = 'Y'
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := store.Set(ctx, "key", []byte("canceled")); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled set: %v", err)
	}
	if err := store.Delete(ctx, "key"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled delete: %v", err)
	}
	read, err = store.Get(t.Context(), "key")
	if err != nil || string(read) != "original" {
		t.Fatalf("returned copy or canceled operation changed cache: %q, %v", read, err)
	}
}

func TestIdleExpirationDoesNotRequireReads(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := memory.New(memory.WithTTL(20 * time.Second))
		defer closeStore(t, store)
		for _, key := range []string{"idle", "read"} {
			if err := store.Set(t.Context(), key, []byte("value")); err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(19 * time.Second)
		synctest.Wait()
		if entries, _ := store.Stats(); entries != 2 {
			t.Fatal("entry expired before its TTL")
		}
		if _, err := store.Get(t.Context(), "read"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Second)
		synctest.Wait()
		// Stats does not itself trigger expiration.
		if entries, bytes := store.Stats(); entries != 0 || bytes != 0 {
			t.Fatalf("idle cleanup retained %d entries, %d bytes", entries, bytes)
		}
		for _, key := range []string{"idle", "read"} {
			if _, err := store.Get(t.Context(), key); !errors.Is(err, cache.ErrNotFound) {
				t.Fatalf("expired entry %q remains readable: %v", key, err)
			}
		}
	})
}

func TestConcurrentOperationsAndClose(t *testing.T) {
	store := memory.New(memory.WithMaxEntries(4), memory.WithMaxBytes(32))
	defer closeStore(t, store)
	var workers sync.WaitGroup
	ready, raceClose := make(chan struct{}, 12), make(chan struct{})
	for i := range 12 {
		workers.Go(func() {
			key := strconv.Itoa(i)
			for pass := range 2 {
				for range 100 {
					if err := store.Set(t.Context(), key, []byte("x")); err != nil && !errors.Is(err, cache.ErrClosed) {
						t.Errorf("concurrent set: %v", err)
					}
					if _, err := store.Get(t.Context(), key); err != nil && !errors.Is(err, cache.ErrClosed) && !errors.Is(err, cache.ErrNotFound) {
						t.Errorf("concurrent get: %v", err)
					}
					if err := store.Delete(t.Context(), key); err != nil && !errors.Is(err, cache.ErrClosed) {
						t.Errorf("concurrent delete: %v", err)
					}
					if entries, bytes := store.Stats(); entries > 4 || bytes > 32 {
						t.Errorf("capacity exceeded: %d entries, %d bytes", entries, bytes)
					}
				}
				if pass == 0 {
					ready <- struct{}{}
					<-raceClose
				}
			}
		})
	}
	for range 12 {
		<-ready
	}
	for range 4 {
		workers.Go(func() { <-raceClose; closeStore(t, store) })
	}
	close(raceClose)
	workers.Wait()
	if entries, bytes := store.Stats(); entries != 0 || bytes != 0 {
		t.Fatalf("closed cache retained %d entries, %d bytes", entries, bytes)
	}
	if _, err := store.Get(t.Context(), "key"); !errors.Is(err, cache.ErrClosed) {
		t.Fatalf("closed get: %v", err)
	}
}

func closeStore(t *testing.T, store io.Closer) {
	t.Helper()
	if err := store.Close(); err != nil {
		t.Errorf("close cache: %v", err)
	}
}
