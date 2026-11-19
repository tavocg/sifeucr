ALTER TABLE budgets ADD COLUMN item_code TEXT NOT NULL DEFAULT '0'
  CHECK (item_code <> '' AND item_code NOT GLOB '*[^0-9]*');
UPDATE budgets SET item_code = CAST(item_number AS TEXT);
ALTER TABLE budgets DROP COLUMN item_number;
ALTER TABLE budgets RENAME COLUMN item_code TO item_number;
