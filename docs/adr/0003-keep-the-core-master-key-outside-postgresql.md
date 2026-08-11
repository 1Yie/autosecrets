# Keep the Core master key outside PostgreSQL

Each Secret Version will use envelope encryption, with its data key protected by a master key loaded from a file outside PostgreSQL. Separating the master key from database data and backups limits the impact of a database-only compromise while preserving unattended self-hosted startup.
