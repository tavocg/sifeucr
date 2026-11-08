-- Preserve existing subjects and relationships without rebuilding accounts.
ALTER TABLE accounts ADD COLUMN auth_id TEXT NOT NULL DEFAULT '';
UPDATE accounts SET auth_id = lower(hex(randomblob(32)));
CREATE UNIQUE INDEX accounts_auth_id_idx ON accounts(auth_id);

-- A single high-water mark survives deletion, including deletion of all accounts.
CREATE TABLE account_subject_sequence (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  last_sub INTEGER NOT NULL
);
INSERT INTO account_subject_sequence (id, last_sub)
SELECT 1, COALESCE(MAX(sub), 0) FROM accounts;

CREATE TRIGGER accounts_subject_sequence AFTER INSERT ON accounts
BEGIN
  SELECT CASE WHEN NEW.sub <= (SELECT last_sub FROM account_subject_sequence WHERE id = 1)
    THEN RAISE(ABORT, 'account subject cannot be reused') END;
  UPDATE account_subject_sequence SET last_sub = NEW.sub WHERE id = 1;
END;

-- Legacy refresh tokens identify accounts only by numeric subject.
DELETE FROM refresh_tokens;
