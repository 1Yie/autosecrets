-- name: Organization :one
SELECT display_name, totp_login_required FROM organization_config WHERE singleton;

-- name: HumanIdentityCount :one
SELECT count(*) FROM admins;

-- name: AcquireBootstrapLock :exec
SELECT pg_advisory_xact_lock(947112301);

-- name: ActiveAdminCount :one
SELECT count(*) FROM admins WHERE role = $1 AND status = $2;

-- name: MemberByUsername :one
SELECT id, username, password_hash, role, status, created_at,
    activated_at, deactivated_at, last_totp_counter
FROM admins WHERE username = $1;

-- name: MemberByID :one
SELECT id, username, password_hash, role, status, created_at,
    activated_at, deactivated_at, last_totp_counter
FROM admins WHERE id = $1;

-- name: InsertOrganizationConfig :exec
INSERT INTO organization_config (singleton, display_name) VALUES (true, $1);

-- name: InsertAdminActive :exec
INSERT INTO admins (id, username, password_hash, role, status, activated_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertAdminPending :exec
INSERT INTO admins (id, username, password_hash, role, status)
VALUES ($1, $2, $3, $4, $5);

-- name: InsertMFAEnrollment :exec
INSERT INTO mfa_enrollments (token_hash, admin_id, wrapped_key, nonces, ciphertext, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: InsertAuditEventLegacy :exec
INSERT INTO audit_events (actor, action, resource, result, correlation_id)
VALUES ($1, $2, $3, $4, $5);

-- name: MFAEnrollmentByToken :one
SELECT admin_id, token_hash, wrapped_key, nonces, ciphertext,
    COALESCE(confirmation_hash, ''), expires_at, verified_at, confirmed_at
FROM mfa_enrollments WHERE token_hash = $1 AND expires_at > $2;

-- name: SelectMFAEnrollmentForVerify :one
SELECT admin_id FROM mfa_enrollments
WHERE token_hash = $1 AND expires_at > $2 AND verified_at IS NULL
FOR UPDATE;

-- name: MarkMFAVerified :exec
UPDATE mfa_enrollments SET verified_at = $2, confirmation_hash = $3 WHERE token_hash = $1;

-- name: InsertRecoveryCode :exec
INSERT INTO recovery_codes (admin_id, code_hash) VALUES ($1, $2);

-- name: SelectMFAEnrollmentForConfirm :one
SELECT admin_id FROM mfa_enrollments
WHERE confirmation_hash = $1 AND expires_at > $2 AND verified_at IS NOT NULL AND confirmed_at IS NULL
FOR UPDATE;

-- name: MarkMFAConfirmed :exec
UPDATE mfa_enrollments SET confirmed_at = $2 WHERE confirmation_hash = $1;

-- name: ActivateMember :exec
UPDATE admins SET status = $2, activated_at = $3 WHERE id = $1 AND status = $4;

-- name: EnableTOTPLoginPolicy :exec
UPDATE organization_config SET totp_login_required = true, updated_at = now() WHERE singleton;

-- name: DisableTOTPLoginPolicy :exec
UPDATE organization_config SET totp_login_required = false, updated_at = now() WHERE singleton;

-- name: PendingMFAEnrollment :one
SELECT EXISTS(SELECT 1 FROM admins WHERE status = $1);

-- name: ResumeMFAEnrollment :execrows
UPDATE mfa_enrollments SET token_hash = $3, expires_at = $4
WHERE admin_id = $1 AND token_hash = $2 AND verified_at IS NULL AND confirmed_at IS NULL;

-- name: HasConfirmedMFA :one
SELECT EXISTS (SELECT 1 FROM mfa_enrollments
    WHERE admin_id = $1 AND verified_at IS NOT NULL AND confirmed_at IS NOT NULL);

-- name: HasAnyConfirmedMFA :one
SELECT EXISTS (SELECT 1 FROM mfa_enrollments
    WHERE admin_id = $1 AND confirmed_at IS NOT NULL);

-- name: DeleteMFAEnrollmentsForMember :exec
DELETE FROM mfa_enrollments WHERE admin_id = $1;

-- name: ConsumeRecoveryCode :execrows
UPDATE recovery_codes SET used_at = $3
WHERE admin_id = $1 AND code_hash = $2 AND used_at IS NULL;

-- name: DeleteRecoveryCodesForMember :exec
DELETE FROM recovery_codes WHERE admin_id = $1;

-- name: UseTOTP :execrows
UPDATE admins SET last_totp_counter = $2
WHERE id = $1 AND (last_totp_counter IS NULL OR last_totp_counter < $2);

-- name: TOTPEnrollmentForMember :one
SELECT admin_id, token_hash, wrapped_key, nonces, ciphertext,
    COALESCE(confirmation_hash, ''), expires_at, verified_at, confirmed_at
FROM mfa_enrollments WHERE admin_id = $1;

-- name: CreateBoundedSession :exec
INSERT INTO sessions (id_hash, admin_id, csrf_token, expires_at, last_activity_at, idle_expires_at, auth_method)
VALUES ($1, $2, $3, $4, now(), $5, $6);

-- name: InsertLoginChallenge :exec
INSERT INTO login_challenges (token_hash, admin_id, source_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: ConsumeLoginChallenge :one
UPDATE login_challenges SET used_at = $3
WHERE token_hash = $1 AND source_hash = $2 AND used_at IS NULL AND expires_at > $3
RETURNING admin_id;

-- name: DeleteExpiredLoginChallenges :exec
DELETE FROM login_challenges WHERE expires_at <= $1 OR used_at IS NOT NULL;

-- name: TouchSessionActivity :exec
UPDATE sessions SET last_activity_at = $2,
    idle_expires_at = LEAST(expires_at, $3) WHERE id_hash = $1;

-- name: UpsertStepUpGrant :exec
INSERT INTO step_up_grants (session_id_hash, granted_at, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (session_id_hash) DO UPDATE
SET granted_at = EXCLUDED.granted_at, expires_at = EXCLUDED.expires_at;

-- name: HasValidStepUpGrant :one
SELECT EXISTS (
  SELECT 1 FROM step_up_grants
  WHERE session_id_hash = $1 AND expires_at > $2
);

-- name: DeleteMemberStepUpGrants :exec
DELETE FROM step_up_grants
WHERE session_id_hash IN (SELECT id_hash FROM sessions WHERE admin_id = $1);

-- name: DeleteMemberSessions :exec
DELETE FROM sessions WHERE admin_id = $1;

-- name: DeleteOtherMemberSessions :exec
DELETE FROM sessions WHERE admin_id = $1 AND id_hash <> $2;

-- name: ExternalIdentityBinding :one
SELECT admin_id, issuer, subject, display_name, created_at
FROM external_identity_binding WHERE singleton;

-- name: InsertExternalIdentityBinding :execrows
INSERT INTO external_identity_binding (singleton, admin_id, issuer, subject, display_name)
VALUES (true, $1, $2, $3, $4)
ON CONFLICT DO NOTHING;

-- name: DeleteExternalIdentityBinding :exec
DELETE FROM external_identity_binding WHERE singleton AND admin_id = $1;

-- name: InsertOIDCTransaction :exec
INSERT INTO oidc_transactions
  (state_hash, purpose, admin_id, nonce, pkce_verifier, return_to, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ConsumeOIDCTransaction :one
UPDATE oidc_transactions SET used_at = $2
WHERE state_hash = $1 AND used_at IS NULL AND expires_at > $2
RETURNING purpose, admin_id, nonce, pkce_verifier, return_to;

-- name: UpdatePassword :exec
UPDATE admins SET password_hash = $2 WHERE id = $1;

-- name: SelectMemberRoleStatus :one
SELECT role, status FROM admins WHERE id = $1 FOR UPDATE;

-- name: CountOtherActiveAdministrators :one
SELECT count(*) FROM admins WHERE role = $1 AND status = $2 AND id <> $3;

-- name: DeactivateMemberRow :exec
UPDATE admins SET status = $2, deactivated_at = now() WHERE id = $1;

-- name: RevokeMemberInvitations :exec
UPDATE member_invitations SET revoked_at = now()
WHERE admin_id = $1 AND used_at IS NULL AND revoked_at IS NULL;
