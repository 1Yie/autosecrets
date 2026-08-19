// Package identity owns the Organization Member domain: Bootstrap, login,
// MFA enrollment, sessions, and password change (ADR-0025: Core business
// code is organized by domain, not by layer).
package identity

import (
	"context"
	"time"

	"autosecrets.dev/core/internal/database"
)

// Repository is the persistence surface the identity domain needs. It is
// satisfied directly by *database.Store; the domain never reaches past these
// methods, so the handler/service split stays testable with a fake.
type Repository interface {
	AdminCount(ctx context.Context) (int, error)
	Organization(ctx context.Context) (*database.Organization, error)
	BootstrapAdministrator(ctx context.Context, codeHash, organizationName,
		memberID, username, passwordHash string, now time.Time, audit database.AuditEvent) error
	MFAEnrollmentByToken(ctx context.Context, tokenHash string, now time.Time) (*database.MFAEnrollment, error)
	CompleteMFAEnrollment(ctx context.Context, tokenHash, confirmationHash string,
		recoveryCodeHashes []string, now time.Time, audit database.AuditEvent) error
	ConfirmMFAEnrollment(ctx context.Context, confirmationHash, currentSessionHash string, now time.Time, audit database.AuditEvent) (*database.Member, error)
	PendingMFAEnrollment(ctx context.Context) (bool, error)
	CreateEnrollmentForMember(ctx context.Context, memberID, tokenHash string,
		wrappedKey, nonces, ciphertext []byte, expiresAt time.Time) error
	HasConfirmedMFA(ctx context.Context, memberID string) (bool, error)
	TOTPEnrollmentForMember(ctx context.Context, memberID string) (*database.MFAEnrollment, error)
	ConsumeRecoveryCode(ctx context.Context, memberID, codeHash string, now time.Time) (bool, error)
	UseTOTP(ctx context.Context, memberID string, counter int64) (bool, error)
	MemberByUsername(ctx context.Context, username string) (*database.Member, error)
	MemberByID(ctx context.Context, id string) (*database.Member, error)
	SessionByID(ctx context.Context, idHash string, now time.Time) (*database.SessionRow, error)
	CreateBoundedSession(ctx context.Context, idHash, memberID, csrfToken string,
		absoluteExpiresAt, idleExpiresAt time.Time, authMethod string) error
	CreateLoginChallenge(ctx context.Context, tokenHash, memberID, sourceHash string, expiresAt time.Time) error
	ConsumeLoginChallenge(ctx context.Context, tokenHash, sourceHash string, now time.Time) (string, error)
	UpsertStepUpGrant(ctx context.Context, sessionIDHash string, grantedAt, expiresAt time.Time) error
	DisableTOTP(ctx context.Context, memberID, currentSessionHash string, audit database.AuditEvent) error
	SetPasswordLoginEnabled(ctx context.Context, memberID, currentSessionHash string, enabled bool, audit database.AuditEvent) error
	ExternalIdentityBinding(ctx context.Context) (*database.ExternalIdentityBinding, error)
	OAuthIdentityBinding(ctx context.Context) (*database.ExternalIdentityBinding, error)
	CreateOIDCTransaction(ctx context.Context, stateHash, purpose, memberID, nonce, verifier, returnTo string, expiresAt time.Time) error
	ConsumeOIDCTransaction(ctx context.Context, stateHash string, now time.Time) (*database.OIDCTransaction, error)
	BindExternalIdentity(ctx context.Context, binding database.ExternalIdentityBinding, currentSessionHash string, audit database.AuditEvent) error
	UnbindExternalIdentity(ctx context.Context, memberID, currentSessionHash string, audit database.AuditEvent) error
	BindOAuthIdentity(ctx context.Context, binding database.ExternalIdentityBinding, currentSessionHash string, audit database.AuditEvent) error
	UnbindOAuthIdentity(ctx context.Context, memberID, currentSessionHash string, audit database.AuditEvent) error
	DeleteSession(ctx context.Context, idHash string) error
	ChangePassword(ctx context.Context, memberID, passwordHash string, audit database.AuditEvent) error
	ChangeUsername(ctx context.Context, memberID, username string, audit database.AuditEvent) error
}

// AuditRecorder is the narrow cross-domain seam the identity domain uses to
// append Audit Events (ADR-0025: domains collaborate through narrow
// interfaces, never by importing another domain's concrete package). A small
// adapter over *database.Store provides the AppendAudit(ctx, event) shape; the
// transactional store methods above record their own Audit Events atomically.
type AuditRecorder interface {
	AppendAudit(ctx context.Context, event database.AuditEvent) error
}
