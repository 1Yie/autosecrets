# Track convergence per Managed Node and Assignment

Core will track Desired and Observed State independently for every Managed Node and Assignment, retain the associated Activation outcome, and derive node and Application health from those records. A single node-level `observed_revision` cannot represent partial success when one Managed Node receives multiple Application and Environment Assignments; keeping Assignment Convergence authoritative makes partial failure visible and gives every aggregated status an explainable source.
