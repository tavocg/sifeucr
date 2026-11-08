package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/postgres/db"
)

func (q *PostgresQuerier) SelectSessionAuthID(ctx context.Context, sub int64, sessionID string) (string, error) {
	return q.queries.SelectSessionAuthID(ctx, db.SelectSessionAuthIDParams{Sub: sub, SessionID: sessionID})
}

func (q *PostgresQuerier) SelectAccountSessions(ctx context.Context, sub, now, beforeCreatedAt int64, beforeID string) ([]database.Session, error) {
	rows, err := q.queries.SelectAccountSessions(ctx, db.SelectAccountSessionsParams{
		Sub:             sub,
		Now:             now,
		BeforeCreatedAt: beforeCreatedAt,
		BeforeID:        beforeID,
	})
	if err != nil {
		return nil, err
	}
	sessions := make([]database.Session, len(rows))
	for i, row := range rows {
		sessions[i] = database.Session(row)
	}
	return sessions, nil
}

func (q *PostgresQuerier) DeleteAccountSession(ctx context.Context, sub int64, sessionID string) error {
	return q.queries.DeleteAccountSession(ctx, db.DeleteAccountSessionParams{Sub: sub, SessionID: sessionID})
}
