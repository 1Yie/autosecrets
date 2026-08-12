# Invite and deactivate Organization Members

Administrators will create Organization Members through revocable, hash-stored, single-use Member Invitations that expire after 24 hours; invited members set their own password, enroll TOTP, and confirm Recovery Codes before activation. Members are deactivated rather than deleted so Audit Events retain an interpretable identity snapshot, deactivation revokes all active credentials, and Core transactionally rejects any role change or deactivation that would leave the Organization without an active Administrator.
