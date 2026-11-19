ALTER TABLE budgets DROP CHECK budgets_chk_1;
ALTER TABLE budgets MODIFY COLUMN item_number MEDIUMTEXT NOT NULL;
ALTER TABLE budgets ADD CHECK (CHAR_LENGTH(item_number) > 0 AND item_number NOT REGEXP '[^0-9]');
