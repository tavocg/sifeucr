package cached

import (
	"context"

	"sifeucr/cache"
	"sifeucr/database"
)

type Querier struct {
	database.Querier
	store      cache.Store
	state      *cacheState
	cacheRead  bool
	invalidate func(keys ...string)
}

var _ database.Querier = (*Querier)(nil)

func (q *Querier) DeleteAccount(ctx context.Context, sub int64) error {
	if err := q.Querier.DeleteAccount(ctx, sub); err != nil {
		return err
	}

	q.invalidate(accountKey(sub))
	return nil
}

func (q *Querier) SelectAccountBySub(ctx context.Context, sub int64) (*database.Account, error) {
	return readThrough(ctx, q, accountKey(sub), func() (*database.Account, error) {
		return q.Querier.SelectAccountBySub(ctx, sub)
	})
}
