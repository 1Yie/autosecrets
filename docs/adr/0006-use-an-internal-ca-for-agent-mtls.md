# Use an internal CA for Agent mTLS

Enrollment will exchange a short-lived, single-use token for a short-lived node certificate after the Agent generates its private key locally. Core will use a dedicated Agent CA, separate from the Secret encryption master key, so each Managed Node can authenticate, renew, and be revoked independently.
