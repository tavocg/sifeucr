package queries

import (
	"context"

	"sifeucr/database"
	"sifeucr/database/mysql/db"
)

func (q *MySQLQuerier) CreateSupplier(ctx context.Context, s database.Supplier) (int64, error) {
	return q.queries.CreateSupplier(ctx, db.CreateSupplierParams{
		IdentityNumber: s.IdentityNumber, SupplierName: s.Name, Description: s.Description,
		Email: s.Email, Phone: s.Phone, Iban: s.IBAN, BudgetID: s.BudgetID,
	})
}
func (q *MySQLQuerier) UpdateSupplier(ctx context.Context, s database.Supplier) (int64, error) {
	params := db.UpdateSupplierParams{
		SupplierID: s.ID, IdentityNumber: s.IdentityNumber, SupplierName: s.Name, Description: s.Description,
		Email: s.Email, Phone: s.Phone, Iban: s.IBAN, BudgetID: s.BudgetID,
	}
	n, err := q.queries.UpdateSupplier(ctx, params)
	if err != nil || n != 0 {
		return n, err
	}
	// MySQL normally reports zero changed rows for an identical update.
	return q.queries.SupplierMatches(ctx, db.SupplierMatchesParams{
		SupplierID: s.ID, BudgetID: s.BudgetID, IdentityNumber: s.IdentityNumber, SupplierName: s.Name, Description: s.Description, Email: s.Email, Phone: s.Phone, Iban: s.IBAN,
	})
}
func (q *MySQLQuerier) ListSuppliers(ctx context.Context) ([]database.Supplier, error) {
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
