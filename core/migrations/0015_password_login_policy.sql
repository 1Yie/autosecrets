-- Password Login Policy: local username-and-password can be closed while an
-- External Identity Binding remains usable for login (ADR-0027).

ALTER TABLE organization_config
ADD COLUMN password_login_enabled boolean NOT NULL DEFAULT true;
