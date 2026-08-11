-- AutoSecrets Core schema, first vertical slice.

CREATE TABLE admins (
  id uuid PRIMARY KEY,
  username text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id_hash text PRIMARY KEY,
  admin_id uuid NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
  csrf_token text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

CREATE TABLE bootstrap_codes (
  code_hash text PRIMARY KEY,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at timestamptz
);

CREATE TABLE audit_events (
  id bigserial PRIMARY KEY,
  actor text NOT NULL,
  action text NOT NULL,
  resource text NOT NULL DEFAULT '',
  result text NOT NULL DEFAULT '',
  correlation_id text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE applications (
  id uuid PRIMARY KEY,
  name text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE environments (
  id uuid PRIMARY KEY,
  application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (application_id, name)
);

CREATE TABLE secrets (
  id uuid PRIMARY KEY,
  application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
  name text NOT NULL,
  retired_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (application_id, environment_id, name)
);

CREATE TABLE secret_versions (
  id uuid PRIMARY KEY,
  secret_id uuid NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
  seq bigint NOT NULL,
  wrapped_key bytea NOT NULL,
  nonce bytea NOT NULL,
  ciphertext bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (secret_id, seq)
);

CREATE TABLE file_bindings (
  id uuid PRIMARY KEY,
  secret_id uuid NOT NULL UNIQUE REFERENCES secrets(id) ON DELETE CASCADE,
  path text NOT NULL,
  uid bigint NOT NULL,
  gid bigint NOT NULL,
  mode bigint NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE drafts (
  id uuid PRIMARY KEY,
  application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
  version bigint NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (application_id, environment_id)
);

CREATE TABLE draft_selections (
  draft_id uuid NOT NULL REFERENCES drafts(id) ON DELETE CASCADE,
  secret_id uuid NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
  version_seq bigint NOT NULL,
  PRIMARY KEY (draft_id, secret_id)
);

CREATE TABLE bundle_revisions (
  id uuid PRIMARY KEY,
  application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
  draft_version bigint NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  created_by text NOT NULL DEFAULT ''
);

CREATE TABLE revision_files (
  revision_id uuid NOT NULL REFERENCES bundle_revisions(id) ON DELETE CASCADE,
  secret_id uuid NOT NULL REFERENCES secrets(id),
  path text NOT NULL,
  uid bigint NOT NULL,
  gid bigint NOT NULL,
  mode bigint NOT NULL,
  version_seq bigint NOT NULL,
  PRIMARY KEY (revision_id, secret_id)
);

CREATE TABLE nodes (
  id uuid PRIMARY KEY,
  name text NOT NULL,
  serial text NOT NULL UNIQUE,
  age_pubkey text NOT NULL,
  cert_pem text NOT NULL,
  cert_expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz,
  desired_etag text NOT NULL DEFAULT '',
  observed_revision text NOT NULL DEFAULT '',
  last_result text NOT NULL DEFAULT ''
);

CREATE TABLE node_groups (
  id uuid PRIMARY KEY,
  name text NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE group_members (
  group_id uuid NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
  node_id uuid NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  PRIMARY KEY (group_id, node_id)
);

CREATE TABLE assignments (
  id uuid PRIMARY KEY,
  group_id uuid NOT NULL REFERENCES node_groups(id) ON DELETE CASCADE,
  revision_id uuid NOT NULL REFERENCES bundle_revisions(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (group_id, revision_id)
);

CREATE TABLE enrollment_tokens (
  token_hash text PRIMARY KEY,
  name text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  used_at timestamptz
);

CREATE INDEX idx_audit_created_at ON audit_events (created_at DESC);
CREATE INDEX idx_nodes_serial ON nodes (serial);
