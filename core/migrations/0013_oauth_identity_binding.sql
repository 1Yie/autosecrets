-- Independent OAuth 2.0 External Identity Binding alongside OIDC.

ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_auth_method_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_auth_method_check
  CHECK (auth_method IN ('local', 'oidc', 'oauth'));

CREATE TABLE oauth_identity_binding (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  admin_id uuid NOT NULL UNIQUE REFERENCES admins(id) ON DELETE CASCADE,
  issuer text NOT NULL,
  subject text NOT NULL,
  display_name text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (issuer, subject)
);
