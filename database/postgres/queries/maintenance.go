package queries

import (
	"context"
	"errors"

	"sifeucr/database"
)

func (q *PostgresQuerier) DeleteExpiredRecords(ctx context.Context, now int64) error {
	// Independent statements release locks between batches and still run if one fails.
	return errors.Join(
		q.queries.DeleteExpiredRefreshTokens(ctx, now),
		database.DeleteExpiredBatches(ctx, now, q.queries.DeleteExpiredRateLimits),
		database.DeleteExpiredBatches(ctx, now, q.queries.DeleteExpiredConsumedRefreshTokens),
	)
}
