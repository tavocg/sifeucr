package queries

import (
	"context"
	"errors"

	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

func (q *MySQLQuerier) DeleteExpiredRecords(ctx context.Context, now int64) error {
	// Independent statements release locks between batches and still run if one fails.
	return errors.Join(
		q.queries.DeleteExpiredLoginRequests(ctx, db.DeleteExpiredLoginRequestsParams{Now: now}),
		q.queries.DeleteExpiredEmailChangeRequests(ctx, db.DeleteExpiredEmailChangeRequestsParams{Now: now}),
		q.queries.DeleteExpiredRefreshTokens(ctx, db.DeleteExpiredRefreshTokensParams{Now: now}),
		database.DeleteExpiredBatches(ctx, now, q.queries.DeleteExpiredRateLimits),
	)
}
