-- Rotatable Secret rotation targets (ADR: Core-driven rotation).
-- A row records that an Administrator rotated a Secret to version_seq;
-- nodes converge to it on their next poll, after which normal
-- keep-old-value behavior resumes.
CREATE TABLE secret_rotations (
    id          bigserial PRIMARY KEY,
    secret_id   uuid NOT NULL REFERENCES secrets (id) ON DELETE CASCADE,
    version_seq bigint NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX secret_rotations_secret_idx ON secret_rotations (secret_id, created_at DESC);
