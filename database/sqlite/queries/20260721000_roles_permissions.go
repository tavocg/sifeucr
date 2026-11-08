package queries

import (
	"context"

	"sifeucr/database/sqlite/db"
)

func (q *SQLiteQuerier) AssignRoleToAccount(ctx context.Context, sub int64, roleKey string) error {
	return q.queries.AssignRoleToAccount(ctx, db.AssignRoleToAccountParams{
		Sub:     sub,
		RoleKey: roleKey,
	})
}

func (q *SQLiteQuerier) SelectAccountRolesBySub(ctx context.Context, sub int64) ([]string, error) {
	return q.queries.SelectAccountRolesBySub(ctx, sub)
}

func (q *SQLiteQuerier) RoleHasPermission(ctx context.Context, roleKey string, permissionKey string) (bool, error) {
	return q.queries.RoleHasPermission(ctx, db.RoleHasPermissionParams{
		RoleKey:       roleKey,
		PermissionKey: permissionKey,
	})
}

func (q *SQLiteQuerier) AccountHasPermission(ctx context.Context, sub int64, permissionKey string) (bool, error) {
	return q.queries.AccountHasPermission(ctx, db.AccountHasPermissionParams{
		Sub:           sub,
		PermissionKey: permissionKey,
	})
}
