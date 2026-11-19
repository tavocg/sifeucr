ALTER TABLE budgets DROP CONSTRAINT budgets_item_number_check;
ALTER TABLE budgets ALTER COLUMN item_number TYPE TEXT USING item_number::text;
ALTER TABLE budgets ADD CHECK (item_number ~ '^[0-9]+$');
