package queries

import (
	"context"
	"fmt"

	"sifeucr/database/mysql/db"
)

func (q *MySQLQuerier) TakeRateLimit(ctx context.Context, scope, key string, now, windowSeconds, maxRequests int64) (int64, error) {
	if windowSeconds <= 0 || maxRequests <= 0 {
		return 0, fmt.Errorf("rate limit window and request limit must be positive")
	}

	var retryAfter int64
	err := q.withTx(ctx, func(q *MySQLQuerier) error {
		if err := q.queries.TakeRateLimit(ctx, db.TakeRateLimitParams{
			Scope:       scope,
			BucketKey:   key,
			StartedAt:   now,
			MaxRequests: maxRequests,
			ExpiresAt:   now + windowSeconds,
		}); err != nil {
			return err
		}

		row, err := q.queries.SelectRateLimit(ctx, db.SelectRateLimitParams{
			Scope:     scope,
			BucketKey: key,
		})
		if err != nil {
			return err
		}
		if row.Requests > row.MaxRequests {
			retryAfter = row.ExpiresAt - now
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return retryAfter, nil
}
