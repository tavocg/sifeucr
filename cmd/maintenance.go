package cmd

import (
	"context"
	"log/slog"
	"time"

	"sifeucr/database"
)

// startDatabaseMaintenance runs bounded batches at startup and once a minute.
// Stop cancels any in-flight cleanup and waits before the database is closed.
func startDatabaseMaintenance(ctx context.Context, db database.Database, logger *slog.Logger) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for ctx.Err() == nil {
			batchCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			err := db.Querier().DeleteExpiredRecords(batchCtx, time.Now().Unix())
			stop()
			if err != nil && ctx.Err() == nil {
				logger.Error("database expiration cleanup failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
