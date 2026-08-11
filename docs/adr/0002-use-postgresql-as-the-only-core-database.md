# Use PostgreSQL as the only Core database

Core will persist identities, metadata, encrypted Secret Versions, Agent state, and audit records in PostgreSQL. Supporting one transactional database avoids divergent locking and migration behavior while providing mature concurrency, constraints, and backup tooling.
