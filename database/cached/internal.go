package cached

import (
	"context"
	"encoding/json"
	"strconv"

	"sifeucr/cache"
)

func accountKey(sub int64) string {
	return "db:account:" + strconv.FormatInt(sub, 10)
}

func setJSON(ctx context.Context, store cache.Store, key string, value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}

	_ = store.Set(ctx, key, data)
}

func getJSON[T any](ctx context.Context, store cache.Store, key string) (T, bool) {
	var zero T

	data, err := store.Get(ctx, key)
	if err != nil {
		return zero, false
	}

	var value T
	err = json.Unmarshal(data, &value)
	if err != nil {
		_ = store.Delete(ctx, key)
		return zero, false
	}

	return value, true
}

func readThrough[T any](ctx context.Context, q *Querier, key string, load func() (T, error)) (T, error) {
	if !q.cacheRead {
		return load()
	}

	shard := q.state.shard(key)
	shard.mu.RLock()
	generation := shard.generation
	cacheKey := q.state.key(key, generation)
	value, ok := getJSON[T](ctx, q.store, cacheKey)
	shard.mu.RUnlock()
	if err := ctx.Err(); err != nil {
		var zero T
		return zero, err
	}
	if ok {
		return value, nil
	}

	// Cache failures fall back to the database; no lock spans a database query.
	value, err := load()
	if err != nil {
		return value, err
	}

	shard.mu.RLock()
	if generation == shard.generation {
		setJSON(ctx, q.store, cacheKey, value)
	}
	shard.mu.RUnlock()
	return value, nil
}
