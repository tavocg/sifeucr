CREATE TABLE IF NOT EXISTS permissions (
  key VARCHAR(128) PRIMARY KEY,
  created_at INTEGER NOT NULL DEFAULT (unixepoch('now'))
);

CREATE TABLE IF NOT EXISTS roles (
  key VARCHAR(128) PRIMARY KEY,
  created_at INTEGER NOT NULL DEFAULT (unixepoch('now'))
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_key VARCHAR(128) NOT NULL REFERENCES roles(key) ON DELETE CASCADE,
  permission_key VARCHAR(128) NOT NULL REFERENCES permissions(key) ON DELETE CASCADE,
  PRIMARY KEY (role_key, permission_key)
);

CREATE TABLE IF NOT EXISTS account_roles (
  sub INTEGER NOT NULL REFERENCES accounts(sub) ON DELETE CASCADE,
  role_key VARCHAR(128) NOT NULL REFERENCES roles(key) ON DELETE CASCADE,
  PRIMARY KEY (sub, role_key)
);

INSERT INTO permissions (key)
VALUES ('perm.change_email')
ON CONFLICT(key) DO NOTHING;

INSERT INTO roles (key)
VALUES ('role.default')
ON CONFLICT(key) DO NOTHING;

INSERT INTO role_permissions (role_key, permission_key)
VALUES ('role.default', 'perm.change_email')
ON CONFLICT(role_key, permission_key) DO NOTHING;
