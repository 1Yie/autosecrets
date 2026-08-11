# Use age-encrypted Recovery Bundles

Core backup tooling will stream the PostgreSQL logical backup and required recovery key material into a package encrypted directly to an Administrator-supplied age recipient, without writing plaintext intermediates. A backup is not considered usable until an automated restore check has validated it.
