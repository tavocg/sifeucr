package queries

import (
	"context"

	"sifeucr/database/postgres/db"
)

func (q *PostgresQuerier) AssignRoleToAccount(ctx context.Context, sub int64, roleKey string) error {
	return q.queries.AssignRoleToAccount(ctx, db.AssignRoleToAccountParams{
		Sub:     sub,
		RoleKey: roleKey,
	})
}

func (q *PostgresQuerier) SelectAccountRolesBySub(ctx context.Context, sub int64) ([]string, error) {
	return q.queries.SelectAccountRolesBySub(ctx, sub)
}

func (q *PostgresQuerier) RoleHasPermission(ctx context.Context, roleKey string, permissionKey string) (bool, error) {
	return q.queries.RoleHasPermission(ctx, db.RoleHasPermissionParams{
		RoleKey:       roleKey,
		PermissionKey: permissionKey,
	})
}

func (q *PostgresQuerier) AccountHasPermission(ctx context.Context, sub int64, permissionKey string) (bool, error) {
	return q.queries.AccountHasPermission(ctx, db.AccountHasPermissionParams{
		Sub:           sub,
		PermissionKey: permissionKey,
	})
}
