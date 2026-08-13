-- Single Administrator, optional local TOTP, and OIDC (ADR-0026).

ALTER TABLE organization_config
  ADD COLUMN totp_login_required boolean NOT NULL DEFAULT false;

ALTER TABLE sessions
  ADD COLUMN auth_method text NOT NULL DEFAULT 'local'
    CHECK (auth_method IN ('local', 'oidc'));

CREATE TABLE login_challenges (
  token_hash text PRIMARY KEY,
  admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
  source_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at timestamptz
);

CREATE TABLE external_identity_binding (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  admin_id uuid NOT NULL UNIQUE REFERENCES admins(id) ON DELETE CASCADE,
  issuer text NOT NULL,
  subject text NOT NULL,
  display_name text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (issuer, subject)
);

CREATE TABLE oidc_transactions (
  state_hash text PRIMARY KEY,
  purpose text NOT NULL CHECK (purpose IN ('login', 'bind')),
  admin_id uuid REFERENCES admins(id) ON DELETE CASCADE,
  nonce text NOT NULL,
  pkce_verifier text NOT NULL,
  return_to text NOT NULL DEFAULT '/dashboard/overview',
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at timestamptz
);

UPDATE organization_config
SET totp_login_required = EXISTS (
  SELECT 1 FROM mfa_enrollments
  WHERE verified_at IS NOT NULL AND confirmed_at IS NOT NULL
);

INSERT INTO audit_events
  (actor, action, resource, result, correlation_id,
   actor_type, actor_id, actor_display,
   resource_type, resource_id, resource_display, outcome)
SELECT
  'system:migration', 'identity.migrated', 'organization:singleton',
  CASE WHEN totp_login_required THEN 'totp_preserved' ELSE 'password_only_enabled' END, '',
  'system', 'migration', 'Schema migration',
  'organization', 'singleton', 'Organization',
  CASE WHEN totp_login_required THEN 'totp_preserved' ELSE 'password_only_enabled' END
FROM organization_config
WHERE singleton;

UPDATE admins
SET status = 'active', activated_at = COALESCE(activated_at, now())
WHERE status = 'pending';

DELETE FROM recovery_codes
WHERE admin_id IN (
  SELECT admin_id FROM mfa_enrollments WHERE confirmed_at IS NULL
);

DELETE FROM mfa_enrollments WHERE confirmed_at IS NULL;

CREATE INDEX idx_login_challenges_expiry ON login_challenges (expires_at);
CREATE INDEX idx_oidc_transactions_expiry ON oidc_transactions (expires_at);
