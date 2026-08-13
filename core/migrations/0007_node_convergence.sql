-- Per-Assignment Convergence (ADR-0015): every Managed Node + Assignment
-- pair keeps its own Desired/Observed Revision, latest Activation outcome,
-- error, and report time. The legacy node-level observed_revision columns
-- remain as compatibility data only.
CREATE TABLE node_convergence (
  node_id uuid NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
  assignment_id uuid NOT NULL REFERENCES assignments (id) ON DELETE CASCADE,
  application_id uuid NOT NULL REFERENCES applications (id) ON DELETE CASCADE,
  environment_id uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
  desired_revision text NOT NULL DEFAULT '',
  observed_revision text NOT NULL DEFAULT '',
  stage text NOT NULL DEFAULT '',
  result text NOT NULL DEFAULT '',
  error text NOT NULL DEFAULT '',
  reported_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (node_id, assignment_id)
);
CREATE INDEX node_convergence_assignment_idx ON node_convergence (assignment_id);
