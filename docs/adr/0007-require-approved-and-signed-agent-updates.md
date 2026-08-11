# Require approved and signed Agent updates

Core will pin an Administrator-approved target Agent version rather than silently following the latest release. An Agent update must verify a signed artifact, replace the executable atomically, restart, and restore the previous executable if the new version cannot start successfully.
