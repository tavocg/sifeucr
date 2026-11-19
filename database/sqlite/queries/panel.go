package queries

import (
	"context"
	"sifeucr/database"
	"sifeucr/database/sqlite/db"
)

func (q *SQLiteQuerier) ClaimSetup(ctx context.Context) (int64, error) {
	return q.queries.ClaimSetup(ctx)
}
func (q *SQLiteQuerier) CreateBudget(ctx context.Context, b database.Budget) error {
	return q.queries.CreateBudget(ctx, db.CreateBudgetParams{Name: b.Name, ItemNumber: b.ItemNumber, StartsAt: b.StartsAt, EndsAt: b.EndsAt, AmountCents: b.AmountCents})
}
func (q *SQLiteQuerier) CreateProcess(ctx context.Context, p database.PurchaseProcess) (int64, error) {
	return q.queries.CreateProcess(ctx, db.CreateProcessParams{ProcessName: p.Name, BudgetID: p.BudgetID, StartsAt: p.StartsAt, EndsAt: p.EndsAt})
}
func (q *SQLiteQuerier) ListBudgets(ctx context.Context) ([]database.Budget, error) {
	rows, err := q.queries.ListBudgets(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]database.Budget, len(rows))
	for i, row := range rows {
		result[i] = database.Budget(row)
	}
	return result, nil
}
func (q *SQLiteQuerier) ListProcesses(ctx context.Context) ([]database.PurchaseProcess, error) {
	rows, err := q.queries.ListProcesses(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]database.PurchaseProcess, len(rows))
	for i, row := range rows {
		result[i] = database.PurchaseProcess(row)
	}
	return result, nil
}
