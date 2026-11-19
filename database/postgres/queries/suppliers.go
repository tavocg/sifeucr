package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/postgres/db"
)

func (q *PostgresQuerier) CreateSupplier(ctx context.Context, s database.Supplier) (int64, error) {
	return q.queries.CreateSupplier(ctx, db.CreateSupplierParams{
		IdentityNumber: s.IdentityNumber, SupplierName: s.Name, Description: s.Description,
		Email: s.Email, Phone: s.Phone, Iban: s.IBAN, BudgetID: s.BudgetID,
	})
}
func (q *PostgresQuerier) UpdateSupplier(ctx context.Context, s database.Supplier) (int64, error) {
	return q.queries.UpdateSupplier(ctx, db.UpdateSupplierParams{
		SupplierID: s.ID, IdentityNumber: s.IdentityNumber, SupplierName: s.Name, Description: s.Description,
		Email: s.Email, Phone: s.Phone, Iban: s.IBAN, BudgetID: s.BudgetID,
	})
}
func (q *PostgresQuerier) ListSuppliers(ctx context.Context) ([]database.Supplier, error) {
	rows, err := q.queries.ListSuppliers(ctx)
	if err != nil {
		return nil, err
	}
	suppliers := make([]database.Supplier, 0, len(rows))
	for _, r := range rows {
		suppliers = append(suppliers, database.Supplier{
			ID: r.ID, IdentityNumber: r.IdentityNumber, Name: r.Name, Description: r.Description,
			Email: r.Email, Phone: r.Phone, IBAN: r.Iban, BudgetID: r.BudgetID, BudgetName: r.BudgetName,
		})
	}
	return suppliers, nil
}
