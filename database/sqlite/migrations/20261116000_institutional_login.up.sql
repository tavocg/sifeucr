-- Stop old application writers before applying this migration.
DROP TABLE account_email_change_requests;
DROP TABLE account_login_requests;
DELETE FROM role_permissions WHERE permission_key = 'perm.change_email';
DELETE FROM permissions WHERE key = 'perm.change_email';
DELETE FROM rate_limits WHERE scope LIKE 'email_change.%' OR scope LIKE 'login.issue.%' OR scope = 'login.verify';
