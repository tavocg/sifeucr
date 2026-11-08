// Package cached wraps database operations with cache-backed reads.
package cached

import (
	"context"
	"time"

	"sifeucr/cache"
	"sifeucr/database"
)

type Database struct {
	next  database.Database
	store cache.Store
	state *cacheState
}

var _ database.Database = (*Database)(nil)

// New coordinates cache reads and invalidations within this wrapper only.
// Share one wrapper per database in a process. Other processes and direct writes
// are not coordinated; their changes become visible after cached entries expire.
// The caller owns the store and must close it if needed.
func New(next database.Database, store cache.Store) database.Database {
	if next == nil || store == nil {
		return next
	}

	return &Database{
		next:  next,
		store: store,
		state: newCacheState(),
	}
}

func (d *Database) Querier() database.Querier {
	return &Querier{
		Querier:    d.next.Querier(),
		store:      d.store,
		state:      d.state,
		cacheRead:  true,
		invalidate: d.invalidate,
	}
}

func (d *Database) WithTx(ctx context.Context, fn func(q database.Querier) error) error {
	pending := make(map[string]struct{})
	err := d.next.WithTx(ctx, func(q database.Querier) error {
		return fn(&Querier{
			Querier: q,
			store:   d.store,
			invalidate: func(keys ...string) {
				for _, key := range keys {
					pending[key] = struct{}{}
				}
			},
		})
	})
	if err != nil {
		return err
	}

	for key := range pending {
		d.invalidate(key)
	}
	return nil
}

func (d *Database) IsErrNotFound(err error) bool {
	return d.next.IsErrNotFound(err)
}

func (d *Database) Close() error {
	return d.next.Close()
}

func (d *Database) invalidate(keys ...string) {
	// A successful write must invalidate even if the request was canceled.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, key := range keys {
		shard := d.state.shard(key)
		shard.mu.Lock()
		oldKey := d.state.key(key, shard.generation)
		shard.generation++
		shard.mu.Unlock()
		// Old generations are unreachable even when deletion fails.
		_ = d.store.Delete(ctx, oldKey)
	}
}
