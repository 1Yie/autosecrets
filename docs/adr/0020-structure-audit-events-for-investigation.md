# Structure Audit Events for investigation

Audit Events will use stable actor, action, resource, and outcome codes with display snapshots, correlation identifiers, redacted metadata, and an Operation Reason where policy requires one. The Audit Interface will record successful, rejected, and business-failed attempts at authenticated high-risk actions, while anonymous invalid-session and CSRF traffic remains in security logs; this keeps investigations filterable and attributable without parsing free-form strings or allowing unauthenticated traffic to flood the append-only evidence store.
