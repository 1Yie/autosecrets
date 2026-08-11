# Separate Web and Agent trust surfaces

One Core service will expose a session-authenticated `/api/v1` management API on the Web hostname and an mTLS-authenticated `/agent/v1` API on a separate Agent hostname. Both hostnames use port 443 and may terminate at the same Compose/Caddy deployment, but their authentication and authorization paths remain independent.
