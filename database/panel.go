package database

import (
	"context"
	"fmt"
)

type Budget struct {
	ID                            int64
	Name                          string
	ItemNumber                    string
	StartsAt, EndsAt, AmountCents int64
}

func (b Budget) Amount() string { return fmt.Sprintf("%d.%02d", b.AmountCents/100, b.AmountCents%100) }

type PurchaseProcess struct {
	ID, BudgetID     int64
	Name, BudgetName string
	StartsAt, EndsAt int64
}

type PanelQuerier interface {
	ClaimSetup(context.Context) (int64, error)
	CreateBudget(context.Context, Budget) error
	CreateProcess(context.Context, PurchaseProcess) (int64, error)
	ListBudgets(context.Context) ([]Budget, error)
	ListProcesses(context.Context) ([]PurchaseProcess, error)
}
