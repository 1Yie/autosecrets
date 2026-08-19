package database

import (
	"context"
	"fmt"
	"time"

	"autosecrets.dev/core/internal/database/gen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	RoleAdministrator = "administrator"
	RoleViewer        = "viewer"

	MemberPending     = "pending"
	MemberActive      = "active"
	MemberDeactivated = "deactivated"
)

// Organization is the one administrative boundary in a self-hosted Core.
type Organization struct {
	DisplayName          string
	TOTPLoginRequired    bool
	PasswordLoginEnabled bool
}

type ExternalIdentityBinding struct {
	MemberID    string
	Issuer      string
	Subject     string
	DisplayName string
	CreatedAt   time.Time
}

type OIDCTransaction struct {
	Purpose      string
	MemberID     string
	Nonce        string
	PKCEVerifier string
	ReturnTo     string
}

func (s *Store) Organization(ctx context.Context) (*Organization, error) {
	row, err := s.q.Organization(ctx)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Organization{
		DisplayName:          row.DisplayName,
		TOTPLoginRequired:    row.TotpLoginRequired,
		PasswordLoginEnabled: row.PasswordLoginEnabled,
	}, nil
}

func (s *Store) HumanIdentityCount(ctx context.Context) (int, error) {
	n, err := s.q.HumanIdentityCount(ctx)
	return int(n), err
}

// ValidateSingleAdministrator rejects compatibility data that cannot be
// represented by the product's single-human identity model.
func (s *Store) ValidateSingleAdministrator(ctx context.Context) error {
	count, err := s.HumanIdentityCount(ctx)
	if err != nil {
		return fmt.Errorf("count human identities: %w", err)
	}
	if count > 1 {
		return fmt.Errorf("single-Administrator invariant violated: found %d human identities; remove extra legacy identities before starting Core", count)
	}
	return nil
}

// Member is the durable identity record. The compatibility Admin alias keeps
// the first vertical slice's internal APIs usable while callers migrate.
type Member struct {
	ID              string
	Username        string
	PasswordHash    string
	Role            string
	Status          string
	CreatedAt       time.Time
	ActivatedAt     *time.Time
	DeactivatedAt   *time.Time
	LastTOTPCounter *int64
}

type Admin = Member

func (s *Store) ActiveAdminCount(ctx context.Context) (int, error) {
	n, err := s.q.ActiveAdminCount(ctx, gen.ActiveAdminCountParams{
		Role: RoleAdministrator, Status: MemberActive,
	})
	return int(n), err
}

func (s *Store) MemberByUsername(ctx context.Context, username string) (*Member, error) {
	r, err := s.q.MemberByUsername(ctx, username)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Member{
		ID: r.ID, Username: r.Username, PasswordHash: r.PasswordHash,
		Role: r.Role, Status: r.Status, CreatedAt: r.CreatedAt,
		ActivatedAt: tsPtr(r.ActivatedAt), DeactivatedAt: tsPtr(r.DeactivatedAt),
		LastTOTPCounter: r.LastTotpCounter,
	}, nil
}

func (s *Store) MemberByID(ctx context.Context, id string) (*Member, error) {
	r, err := s.q.MemberByID(ctx, id)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Member{
		ID: r.ID, Username: r.Username, PasswordHash: r.PasswordHash,
		Role: r.Role, Status: r.Status, CreatedAt: r.CreatedAt,
		ActivatedAt: tsPtr(r.ActivatedAt), DeactivatedAt: tsPtr(r.DeactivatedAt),
		LastTOTPCounter: r.LastTotpCounter,
	}, nil
}

// MFAEnrollment is encrypted at rest by Core's MasterKey. The store never
// interprets the TOTP secret; it only controls the one-time state machine.
type MFAEnrollment struct {
	MemberID         string
	TokenHash        string
	WrappedKey       []byte
	Nonces           []byte
	Ciphertext       []byte
	ConfirmationHash string
	ExpiresAt        time.Time
	VerifiedAt       *time.Time
	ConfirmedAt      *time.Time
}

// BootstrapAdministrator consumes the Bootstrap Code and creates the active,
// single Administrator. TOTP remains disabled until a later enrollment.
func (s *Store) BootstrapAdministrator(ctx context.Context, codeHash, organizationName,
	memberID, username, passwordHash string, now time.Time, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.AcquireBootstrapLock(ctx); err != nil {
		return err
	}
	members, err := q.AdminCount(ctx)
	if err != nil {
		return err
	}
	if members > 0 {
		return ErrConflict
	}
	n, err := q.ConsumeBootstrapCode(ctx, gen.ConsumeBootstrapCodeParams{
		CodeHash: codeHash, UsedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotFound
	}
	if err := q.InsertOrganizationConfig(ctx, organizationName); err != nil {
		return err
	}
	if err := q.InsertAdminActive(ctx, gen.InsertAdminActiveParams{
		ID: memberID, Username: username, PasswordHash: passwordHash,
		Role: RoleAdministrator, Status: MemberActive,
		ActivatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) MFAEnrollmentByToken(ctx context.Context, tokenHash string, now time.Time) (*MFAEnrollment, error) {
	r, err := s.q.MFAEnrollmentByToken(ctx, gen.MFAEnrollmentByTokenParams{TokenHash: tokenHash, ExpiresAt: now})
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &MFAEnrollment{
		MemberID: r.AdminID, TokenHash: r.TokenHash, WrappedKey: r.WrappedKey,
		Nonces: r.Nonces, Ciphertext: r.Ciphertext, ConfirmationHash: r.ConfirmationHash,
		ExpiresAt: r.ExpiresAt, VerifiedAt: tsPtr(r.VerifiedAt), ConfirmedAt: tsPtr(r.ConfirmedAt),
	}, nil
}

// CompleteMFAEnrollment records a successfully verified TOTP secret and
// stores only recovery-code hashes. It returns ErrConflict if the one-time
// verification was already consumed.
func (s *Store) CompleteMFAEnrollment(ctx context.Context, tokenHash, confirmationHash string,
	recoveryCodeHashes []string, now time.Time, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	memberID, err := q.SelectMFAEnrollmentForVerify(ctx, gen.SelectMFAEnrollmentForVerifyParams{
		TokenHash: tokenHash, ExpiresAt: now,
	})
	if err != nil {
		return mapNoRows(err)
	}
	if err := q.MarkMFAVerified(ctx, gen.MarkMFAVerifiedParams{
		TokenHash: tokenHash, VerifiedAt: pgtype.Timestamptz{Time: now, Valid: true}, ConfirmationHash: &confirmationHash,
	}); err != nil {
		return err
	}
	for _, hash := range recoveryCodeHashes {
		if err := q.InsertRecoveryCode(ctx, gen.InsertRecoveryCodeParams{AdminID: memberID, CodeHash: hash}); err != nil {
			return err
		}
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConfirmMFAEnrollment enables the Organization's TOTP Login Policy only
// after Recovery Codes were explicitly acknowledged.
func (s *Store) ConfirmMFAEnrollment(ctx context.Context, confirmationHash, currentSessionHash string, now time.Time, audit AuditEvent) (*Member, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	memberID, err := q.SelectMFAEnrollmentForConfirm(ctx, gen.SelectMFAEnrollmentForConfirmParams{
		ConfirmationHash: &confirmationHash, ExpiresAt: now,
	})
	if err != nil {
		return nil, mapNoRows(err)
	}
	if err := q.MarkMFAConfirmed(ctx, gen.MarkMFAConfirmedParams{
		ConfirmationHash: &confirmationHash, ConfirmedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return nil, err
	}
	if err := q.EnableTOTPLoginPolicy(ctx); err != nil {
		return nil, err
	}
	if err := q.DeleteMemberStepUpGrants(ctx, memberID); err != nil {
		return nil, err
	}
	if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{
		AdminID: memberID, IDHash: currentSessionHash,
	}); err != nil {
		return nil, err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return nil, err
	}
	member, err := s.memberByID(ctx, q, memberID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return member, nil
}

func (s *Store) PendingMFAEnrollment(ctx context.Context) (bool, error) {
	return s.q.PendingMFAEnrollment(ctx, MemberPending)
}

// ResumeMFAEnrollment validates a pending member's password in the app layer
// then swaps the browser-facing token for a fresh one without rotating the
// already enrolled TOTP secret.
func (s *Store) ResumeMFAEnrollment(ctx context.Context, memberID, oldTokenHash, newTokenHash string, expiresAt time.Time) error {
	n, err := s.q.ResumeMFAEnrollment(ctx, gen.ResumeMFAEnrollmentParams{
		AdminID: memberID, TokenHash: oldTokenHash, TokenHash_2: newTokenHash, ExpiresAt: expiresAt,
	})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}

// CreateEnrollmentForMember starts a fresh MFA enrollment for an existing
// active member (legacy accounts that predate mandatory TOTP). Any previous
// unverified enrollment is replaced; a confirmed enrollment is never
// overwritten.
func (s *Store) CreateEnrollmentForMember(ctx context.Context, memberID, tokenHash string,
	wrappedKey, nonces, ciphertext []byte, expiresAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	confirmed, err := q.HasAnyConfirmedMFA(ctx, memberID)
	if err != nil {
		return err
	}
	if confirmed {
		return ErrConflict
	}
	if err := q.DeleteMFAEnrollmentsForMember(ctx, memberID); err != nil {
		return err
	}
	if err := q.InsertMFAEnrollment(ctx, gen.InsertMFAEnrollmentParams{
		TokenHash: tokenHash, AdminID: memberID,
		WrappedKey: wrappedKey, Nonces: nonces, Ciphertext: ciphertext, ExpiresAt: expiresAt,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// HasConfirmedMFA reports whether the member completed TOTP enrollment.
func (s *Store) HasConfirmedMFA(ctx context.Context, memberID string) (bool, error) {
	return s.q.HasConfirmedMFA(ctx, memberID)
}

// ConsumeRecoveryCode atomically burns one code. Callers must normalize and
// hash it before invoking this method.
func (s *Store) ConsumeRecoveryCode(ctx context.Context, memberID, codeHash string, now time.Time) (bool, error) {
	n, err := s.q.ConsumeRecoveryCode(ctx, gen.ConsumeRecoveryCodeParams{
		AdminID: memberID, CodeHash: codeHash, UsedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) ReplaceRecoveryCodes(ctx context.Context, memberID string, hashes []string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.DeleteRecoveryCodesForMember(ctx, memberID); err != nil {
		return err
	}
	for _, hash := range hashes {
		if err := q.InsertRecoveryCode(ctx, gen.InsertRecoveryCodeParams{AdminID: memberID, CodeHash: hash}); err != nil {
			return err
		}
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UseTOTP(ctx context.Context, memberID string, counter int64) (bool, error) {
	n, err := s.q.UseTOTP(ctx, gen.UseTOTPParams{ID: memberID, LastTotpCounter: &counter})
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

func (s *Store) TOTPEnrollmentForMember(ctx context.Context, memberID string) (*MFAEnrollment, error) {
	r, err := s.q.TOTPEnrollmentForMember(ctx, memberID)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &MFAEnrollment{
		MemberID: r.AdminID, TokenHash: r.TokenHash, WrappedKey: r.WrappedKey,
		Nonces: r.Nonces, Ciphertext: r.Ciphertext, ConfirmationHash: r.ConfirmationHash,
		ExpiresAt: r.ExpiresAt, VerifiedAt: tsPtr(r.VerifiedAt), ConfirmedAt: tsPtr(r.ConfirmedAt),
	}, nil
}

func (s *Store) CreateBoundedSession(ctx context.Context, idHash, memberID, csrfToken string,
	absoluteExpiresAt, idleExpiresAt time.Time, authMethod string) error {
	return s.q.CreateBoundedSession(ctx, gen.CreateBoundedSessionParams{
		IDHash: idHash, AdminID: memberID, CsrfToken: csrfToken,
		ExpiresAt: absoluteExpiresAt, IdleExpiresAt: idleExpiresAt, AuthMethod: authMethod,
	})
}

func (s *Store) CreateLoginChallenge(ctx context.Context, tokenHash, memberID, sourceHash string, expiresAt time.Time) error {
	_ = s.q.DeleteExpiredLoginChallenges(ctx, time.Now())
	return s.q.InsertLoginChallenge(ctx, gen.InsertLoginChallengeParams{
		TokenHash: tokenHash, AdminID: memberID, SourceHash: sourceHash, ExpiresAt: expiresAt,
	})
}

func (s *Store) ConsumeLoginChallenge(ctx context.Context, tokenHash, sourceHash string, now time.Time) (string, error) {
	id, err := s.q.ConsumeLoginChallenge(ctx, gen.ConsumeLoginChallengeParams{
		TokenHash: tokenHash, SourceHash: sourceHash,
		UsedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	return id, mapNoRows(err)
}

func (s *Store) UpsertStepUpGrant(ctx context.Context, sessionIDHash string, grantedAt, expiresAt time.Time) error {
	return s.q.UpsertStepUpGrant(ctx, gen.UpsertStepUpGrantParams{
		SessionIDHash: sessionIDHash, GrantedAt: grantedAt, ExpiresAt: expiresAt,
	})
}

func (s *Store) HasValidStepUpGrant(ctx context.Context, sessionIDHash string, now time.Time) (bool, error) {
	return s.q.HasValidStepUpGrant(ctx, gen.HasValidStepUpGrantParams{
		SessionIDHash: sessionIDHash, ExpiresAt: now,
	})
}

func (s *Store) DisableTOTP(ctx context.Context, memberID, currentSessionHash string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DisableTOTPLoginPolicy(ctx); err != nil {
		return err
	}
	if err := q.DeleteMFAEnrollmentsForMember(ctx, memberID); err != nil {
		return err
	}
	if err := q.DeleteRecoveryCodesForMember(ctx, memberID); err != nil {
		return err
	}
	if err := q.DeleteMemberStepUpGrants(ctx, memberID); err != nil {
		return err
	}
	if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{AdminID: memberID, IDHash: currentSessionHash}); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SetPasswordLoginEnabled(ctx context.Context, memberID, currentSessionHash string, enabled bool, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.SetPasswordLoginEnabled(ctx, enabled); err != nil {
		return err
	}
	if !enabled {
		if err := q.DeleteMemberStepUpGrants(ctx, memberID); err != nil {
			return err
		}
		if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{
			AdminID: memberID, IDHash: currentSessionHash,
		}); err != nil {
			return err
		}
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) TouchSessionActivity(ctx context.Context, idHash string, now, idleExpiresAt time.Time) error {
	return s.q.TouchSessionActivity(ctx, gen.TouchSessionActivityParams{
		IDHash: idHash, LastActivityAt: now, IdleExpiresAt: idleExpiresAt,
	})
}

func (s *Store) DeleteMemberSessions(ctx context.Context, memberID string) error {
	return s.q.DeleteMemberSessions(ctx, memberID)
}

func (s *Store) ExternalIdentityBinding(ctx context.Context) (*ExternalIdentityBinding, error) {
	row, err := s.q.ExternalIdentityBinding(ctx)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &ExternalIdentityBinding{
		MemberID: row.AdminID, Issuer: row.Issuer, Subject: row.Subject,
		DisplayName: row.DisplayName, CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Store) OAuthIdentityBinding(ctx context.Context) (*ExternalIdentityBinding, error) {
	row, err := s.q.OAuthIdentityBinding(ctx)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &ExternalIdentityBinding{
		MemberID: row.AdminID, Issuer: row.Issuer, Subject: row.Subject,
		DisplayName: row.DisplayName, CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Store) CreateOIDCTransaction(ctx context.Context, stateHash, purpose, memberID, nonce, verifier, returnTo string, expiresAt time.Time) error {
	adminID := pgtype.UUID{}
	if memberID != "" {
		if err := adminID.Scan(memberID); err != nil {
			return err
		}
	}
	return s.q.InsertOIDCTransaction(ctx, gen.InsertOIDCTransactionParams{
		StateHash: stateHash, Purpose: purpose, AdminID: adminID, Nonce: nonce,
		PkceVerifier: verifier, ReturnTo: returnTo, ExpiresAt: expiresAt,
	})
}

func (s *Store) ConsumeOIDCTransaction(ctx context.Context, stateHash string, now time.Time) (*OIDCTransaction, error) {
	row, err := s.q.ConsumeOIDCTransaction(ctx, gen.ConsumeOIDCTransactionParams{
		StateHash: stateHash, UsedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return nil, mapNoRows(err)
	}
	memberID := ""
	if row.AdminID.Valid {
		memberID = uuid.UUID(row.AdminID.Bytes).String()
	}
	return &OIDCTransaction{
		Purpose: row.Purpose, MemberID: memberID, Nonce: row.Nonce,
		PKCEVerifier: row.PkceVerifier, ReturnTo: row.ReturnTo,
	}, nil
}

func (s *Store) BindExternalIdentity(ctx context.Context, binding ExternalIdentityBinding, currentSessionHash string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	rows, err := q.InsertExternalIdentityBinding(ctx, gen.InsertExternalIdentityBindingParams{
		AdminID: binding.MemberID, Issuer: binding.Issuer, Subject: binding.Subject, DisplayName: binding.DisplayName,
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrConflict
	}
	if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{AdminID: binding.MemberID, IDHash: currentSessionHash}); err != nil {
		return err
	}
	if err := q.DeleteMemberStepUpGrants(ctx, binding.MemberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UnbindExternalIdentity(ctx context.Context, memberID, currentSessionHash string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteExternalIdentityBinding(ctx, memberID); err != nil {
		return err
	}
	if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{AdminID: memberID, IDHash: currentSessionHash}); err != nil {
		return err
	}
	if err := q.DeleteMemberStepUpGrants(ctx, memberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) BindOAuthIdentity(ctx context.Context, binding ExternalIdentityBinding, currentSessionHash string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	rows, err := q.InsertOAuthIdentityBinding(ctx, gen.InsertOAuthIdentityBindingParams{
		AdminID: binding.MemberID, Issuer: binding.Issuer, Subject: binding.Subject, DisplayName: binding.DisplayName,
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrConflict
	}
	if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{AdminID: binding.MemberID, IDHash: currentSessionHash}); err != nil {
		return err
	}
	if err := q.DeleteMemberStepUpGrants(ctx, binding.MemberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UnbindOAuthIdentity(ctx context.Context, memberID, currentSessionHash string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)
	if err := q.DeleteOAuthIdentityBinding(ctx, memberID); err != nil {
		return err
	}
	if err := q.DeleteOtherMemberSessions(ctx, gen.DeleteOtherMemberSessionsParams{AdminID: memberID, IDHash: currentSessionHash}); err != nil {
		return err
	}
	if err := q.DeleteMemberStepUpGrants(ctx, memberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ChangePassword updates the password hash, revokes every Session (and with
// them their Step-up Grants via cascade), and appends the Audit Event in the
// same transaction (implementation-plan: security-relevant changes are
// audited atomically).
func (s *Store) ChangePassword(ctx context.Context, memberID, passwordHash string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.UpdatePassword(ctx, gen.UpdatePasswordParams{ID: memberID, PasswordHash: passwordHash}); err != nil {
		return err
	}
	if err := q.DeleteMemberSessions(ctx, memberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ChangeUsername updates the local login name, revokes every Session, and
// appends the Audit Event in the same transaction.
func (s *Store) ChangeUsername(ctx context.Context, memberID, username string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	if err := q.UpdateUsername(ctx, gen.UpdateUsernameParams{ID: memberID, Username: username}); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	if err := q.DeleteMemberSessions(ctx, memberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) DeactivateMember(ctx context.Context, memberID string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := s.q.WithTx(tx)

	row, err := q.SelectMemberRoleStatus(ctx, memberID)
	if err != nil {
		return mapNoRows(err)
	}
	if row.Status == MemberDeactivated {
		return ErrConflict
	}
	if row.Role == RoleAdministrator {
		remaining, err := q.CountOtherActiveAdministrators(ctx, gen.CountOtherActiveAdministratorsParams{
			Role: RoleAdministrator, Status: MemberActive, ID: memberID,
		})
		if err != nil {
			return err
		}
		if remaining == 0 {
			return fmt.Errorf("%w: last active administrator", ErrConflict)
		}
	}
	if err := q.DeactivateMemberRow(ctx, gen.DeactivateMemberRowParams{ID: memberID, Status: MemberDeactivated}); err != nil {
		return err
	}
	if err := q.DeleteMemberSessions(ctx, memberID); err != nil {
		return err
	}
	if err := q.RevokeMemberInvitations(ctx, memberID); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, q, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) memberByID(ctx context.Context, q *gen.Queries, id string) (*Member, error) {
	r, err := q.MemberByID(ctx, id)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &Member{
		ID: r.ID, Username: r.Username, PasswordHash: r.PasswordHash,
		Role: r.Role, Status: r.Status, CreatedAt: r.CreatedAt,
		ActivatedAt: tsPtr(r.ActivatedAt), DeactivatedAt: tsPtr(r.DeactivatedAt),
		LastTOTPCounter: r.LastTotpCounter,
	}, nil
}

func appendAuditTx(ctx context.Context, q *gen.Queries, event AuditEvent) error {
	if event.Actor == "" {
		event.Actor = "system"
	}
	return q.InsertAuditEventLegacy(ctx, gen.InsertAuditEventLegacyParams{
		Actor: event.Actor, Action: event.Action, Resource: event.Resource,
		Result: event.Result, CorrelationID: event.CorrelationID,
	})
}
