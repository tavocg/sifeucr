-- name: AssignRoleToAccount :exec
INSERT INTO account_roles (sub, role_key)
VALUES (?, ?)
ON DUPLICATE KEY UPDATE sub = VALUES(sub);

-- name: SelectAccountRolesBySub :many
SELECT role_key
FROM account_roles
WHERE sub = ?
ORDER BY role_key;

-- name: RoleHasPermission :one
SELECT EXISTS (
  SELECT 1
  FROM role_permissions
  WHERE role_key = ? AND permission_key = ?
) AS has_permission;

-- name: AccountHasPermission :one
SELECT EXISTS (
  SELECT 1
  FROM account_roles ar
  JOIN role_permissions rp ON rp.role_key = ar.role_key
  WHERE ar.sub = sqlc.arg(sub) AND rp.permission_key = sqlc.arg(permission_key)
) AS has_permission;
