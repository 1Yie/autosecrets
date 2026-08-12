-- Organization Member identity, mandatory MFA, bounded Sessions, and
-- server-side Step-up Grants (ADR-0023, ADR-0024).

CREATE TABLE organization_config (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  display_name text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE admins
  ADD COLUMN role text NOT NULL DEFAULT 'administrator'
    CHECK (role IN ('administrator', 'viewer')),
  ADD COLUMN status text NOT NULL DEFAULT 'active'
    CHECK (status IN ('pending', 'active', 'deactivated')),
  ADD COLUMN activated_at timestamptz,
  ADD COLUMN deactivated_at timestamptz,
  ADD COLUMN last_totp_counter bigint;

UPDATE admins SET activated_at = created_at WHERE status = 'active';

INSERT INTO organization_config (display_name)
SELECT 'AutoSecrets'
WHERE EXISTS (SELECT 1 FROM admins)
ON CONFLICT (singleton) DO NOTHING;

CREATE TABLE mfa_enrollments (
  token_hash text PRIMARY KEY,
  admin_id uuid NOT NULL UNIQUE REFERENCES admins(id) ON DELETE CASCADE,
  wrapped_key bytea NOT NULL,
  nonces bytea NOT NULL,
  ciphertext bytea NOT NULL,
  confirmation_hash text UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  verified_at timestamptz,
  confirmed_at timestamptz
);

CREATE TABLE recovery_codes (
  admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
  code_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  used_at timestamptz,
  PRIMARY KEY (admin_id, code_hash)
);

ALTER TABLE sessions
  ADD COLUMN last_activity_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN idle_expires_at timestamptz NOT NULL DEFAULT (now() + interval '30 minutes');

UPDATE sessions
SET idle_expires_at = LEAST(expires_at, created_at + interval '30 minutes'),
    last_activity_at = created_at;

CREATE TABLE step_up_grants (
  session_id_hash text PRIMARY KEY REFERENCES sessions(id_hash) ON DELETE CASCADE,
  granted_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

CREATE TABLE member_invitations (
  token_hash text PRIMARY KEY,
  admin_id uuid NOT NULL UNIQUE REFERENCES admins(id) ON DELETE CASCADE,
  created_by uuid NOT NULL REFERENCES admins(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  revoked_at timestamptz
);

CREATE INDEX idx_sessions_admin ON sessions (admin_id);
CREATE INDEX idx_sessions_expiry ON sessions (expires_at, idle_expires_at);
CREATE INDEX idx_recovery_codes_admin_unused ON recovery_codes (admin_id) WHERE used_at IS NULL;
