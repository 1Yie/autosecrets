---
status: superseded by ADR-0026
---

# Require MFA and bounded Sessions

Every active Organization Member will authenticate with a password and TOTP or a single-use Recovery Code; completing full MFA issues a Session with a 12-hour absolute lifetime, a 30-minute idle lifetime, and a five-minute server-side Step-up Grant. Session renewal requires full MFA and issues a new Session, while logout, expiry, password changes, and member deactivation revoke applicable Sessions and Step-up Grants. The first Administrator remains pending until TOTP enrollment and one-time Recovery Code confirmation are complete, so Bootstrap cannot leave an active password-only identity.
