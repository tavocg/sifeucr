CREATE TABLE IF NOT EXISTS permissions (
  `key` VARCHAR(128) PRIMARY KEY,
  created_at BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
);

CREATE TABLE IF NOT EXISTS roles (
  `key` VARCHAR(128) PRIMARY KEY,
  created_at BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP())
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_key VARCHAR(128) NOT NULL,
  permission_key VARCHAR(128) NOT NULL,
  PRIMARY KEY (role_key, permission_key),
  CONSTRAINT role_permissions_role_fk FOREIGN KEY (role_key) REFERENCES roles(`key`) ON DELETE CASCADE,
  CONSTRAINT role_permissions_permission_fk FOREIGN KEY (permission_key) REFERENCES permissions(`key`) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS account_roles (
  sub BIGINT NOT NULL,
  role_key VARCHAR(128) NOT NULL,
  PRIMARY KEY (sub, role_key),
  CONSTRAINT account_roles_account_fk FOREIGN KEY (sub) REFERENCES accounts(sub) ON DELETE CASCADE,
  CONSTRAINT account_roles_role_fk FOREIGN KEY (role_key) REFERENCES roles(`key`) ON DELETE CASCADE
);

INSERT INTO permissions (`key`)
VALUES ('perm.change_email')
ON DUPLICATE KEY UPDATE `key` = VALUES(`key`);

INSERT INTO roles (`key`)
VALUES ('role.default')
ON DUPLICATE KEY UPDATE `key` = VALUES(`key`);

INSERT INTO role_permissions (role_key, permission_key)
VALUES ('role.default', 'perm.change_email')
ON DUPLICATE KEY UPDATE role_key = VALUES(role_key);
