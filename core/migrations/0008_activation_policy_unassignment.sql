-- Activation Policy and persistent Unassignment (ADR-0022).
CREATE TABLE activation_policies (
  environment_id uuid PRIMARY KEY REFERENCES environments (id) ON DELETE CASCADE,
  action text NOT NULL CHECK (action IN ('none', 'reload', 'restart')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE activation_policy_units (
  environment_id uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
  position smallint NOT NULL CHECK (position BETWEEN 1 AND 5),
  unit_name text NOT NULL,
  PRIMARY KEY (environment_id, position)
);

CREATE TABLE unassignment_tasks (
  assignment_id uuid NOT NULL REFERENCES assignments (id) ON DELETE CASCADE,
  node_id uuid NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'cleaned', 'failed', 'offline', 'cleanup_unconfirmed')),
  error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (assignment_id, node_id)
);

-- A removed Assignment remains as an audit tombstone without blocking a
-- future Assignment of the same bundle to the same group.
DROP INDEX assignments_group_bundle_uniq;
CREATE UNIQUE INDEX assignments_group_bundle_active_uniq
  ON assignments (group_id, application_id, environment_id) WHERE status = 'active';
