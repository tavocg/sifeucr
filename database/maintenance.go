package database

import "context"

// DeleteExpiredBatches drains expired rows in bounded batches until none remain.
// The caller's context limits total cleanup time; deleteBatch must bound each batch.
func DeleteExpiredBatches(ctx context.Context, now int64, deleteBatch func(context.Context, int64) (int64, error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		deleted, err := deleteBatch(ctx, now)
		if err != nil || deleted == 0 {
			return err
		}
	}
}
