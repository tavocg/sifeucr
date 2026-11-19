CREATE TABLE suppliers (
  id INTEGER PRIMARY KEY,
  identity_number VARCHAR(64) NOT NULL,
  name VARCHAR(160) NOT NULL,
  description TEXT NOT NULL,
  email VARCHAR(254) NOT NULL,
  phone VARCHAR(64) NOT NULL,
  iban VARCHAR(64) NOT NULL,
  budget_id INTEGER NOT NULL,
  FOREIGN KEY (budget_id) REFERENCES budgets(id)
);
