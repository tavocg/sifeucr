package database

import "context"

type Supplier struct {
	ID             int64  `json:"id"`
	IdentityNumber string `json:"identity_number"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	IBAN           string `json:"iban"`
	BudgetID       int64  `json:"budget_id"`
	BudgetName     string `json:"budget_name"`
}
type SupplierQuerier interface {
	CreateSupplier(context.Context, Supplier) (int64, error)
	UpdateSupplier(context.Context, Supplier) (int64, error)
	ListSuppliers(context.Context) ([]Supplier, error)
}
