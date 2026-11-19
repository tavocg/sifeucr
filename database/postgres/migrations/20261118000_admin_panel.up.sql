INSERT INTO roles (key) VALUES ('role.admin');
INSERT INTO permissions (key) VALUES ('perm.admin');
INSERT INTO role_permissions (role_key, permission_key) VALUES ('role.admin', 'perm.admin');

-- Existing installations already have a first account.
INSERT INTO account_roles (sub, role_key)
SELECT sub, 'role.admin' FROM accounts ORDER BY created_at, sub LIMIT 1;
CREATE TABLE pending_setup (id INTEGER PRIMARY KEY);
INSERT INTO pending_setup (id) SELECT 1 WHERE NOT EXISTS (SELECT 1 FROM accounts);

CREATE TABLE budgets (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(160) NOT NULL,
  item_number BIGINT NOT NULL CHECK (item_number > 0),
  starts_at BIGINT NOT NULL,
  ends_at BIGINT NOT NULL CHECK (ends_at > starts_at)
);
CREATE TABLE purchase_processes (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(160) NOT NULL,
  budget_id BIGINT NOT NULL,
  FOREIGN KEY (budget_id) REFERENCES budgets(id)
);
