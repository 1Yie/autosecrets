-- The cyclic candidate rotation (keep-old-value) feature was removed by
-- product decision 2026-08: updating a Secret means creating a new Version,
-- never cycling between old candidates.
DROP TABLE IF EXISTS secret_rotations;
