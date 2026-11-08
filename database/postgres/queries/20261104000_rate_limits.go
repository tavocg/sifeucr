package queries

import (
	"context"
	"fmt"

	"sifeucr/database/postgres/db"
)

func (q *PostgresQuerier) TakeRateLimit(ctx context.Context, scope, key string, now, windowSeconds, maxRequests int64) (int64, error) {
	if windowSeconds <= 0 || maxRequests <= 0 {
		return 0, fmt.Errorf("rate limit window and request limit must be positive")
	}
	row, err := q.queries.TakeRateLimit(ctx, db.TakeRateLimitParams{
		Scope:       scope,
		BucketKey:   key,
		StartedAt:   now,
		ExpiresAt:   now + windowSeconds,
		MaxRequests: maxRequests,
	})
	if err != nil {
		return 0, err
	}
	if row.Requests > row.MaxRequests {
		return row.ExpiresAt - now, nil
	}
	return 0, nil
}
