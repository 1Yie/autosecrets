package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	DisplayName string
}

func (s *Store) Organization(ctx context.Context) (*Organization, error) {
	var organization Organization
	err := s.pool.QueryRow(ctx, `SELECT display_name FROM organization_config WHERE singleton`).Scan(&organization.DisplayName)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &organization, nil
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
	var count int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM admins WHERE role = $1 AND status = $2`, RoleAdministrator, MemberActive).Scan(&count)
	return count, err
}

func (s *Store) MemberByUsername(ctx context.Context, username string) (*Member, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, username, password_hash, role, status, created_at,
		activated_at, deactivated_at, last_totp_counter
		FROM admins WHERE username = $1`, username)
	return scanMember(row)
}

func (s *Store) MemberByID(ctx context.Context, id string) (*Member, error) {
	row := s.pool.QueryRow(ctx, `SELECT id, username, password_hash, role, status, created_at,
		activated_at, deactivated_at, last_totp_counter
		FROM admins WHERE id = $1`, id)
	return scanMember(row)
}

func scanMember(row pgx.Row) (*Member, error) {
	member := &Member{}
	if err := row.Scan(&member.ID, &member.Username, &member.PasswordHash, &member.Role,
		&member.Status, &member.CreatedAt, &member.ActivatedAt, &member.DeactivatedAt,
		&member.LastTOTPCounter); err != nil {
		return nil, mapNoRows(err)
	}
	return member, nil
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

// StartBootstrapEnrollment atomically consumes the Bootstrap Code, creates
// the Organization and pending first Administrator, and persists the
// encrypted TOTP enrollment. A transaction advisory lock prevents two
// independently issued Bootstrap Codes from producing two first members.
func (s *Store) StartBootstrapEnrollment(ctx context.Context, codeHash, organizationName,
	memberID, username, passwordHash, enrollmentTokenHash string,
	wrappedKey, nonces, ciphertext []byte, expiresAt time.Time, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(947112301)`); err != nil {
		return err
	}
	var members int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&members); err != nil {
		return err
	}
	if members > 0 {
		return ErrConflict
	}
	tag, err := tx.Exec(ctx, `UPDATE bootstrap_codes SET used_at = now()
		WHERE code_hash = $1 AND used_at IS NULL AND expires_at > now()`, codeHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `INSERT INTO organization_config (singleton, display_name) VALUES (true, $1)`, organizationName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO admins (id, username, password_hash, role, status)
		VALUES ($1, $2, $3, $4, $5)`, memberID, username, passwordHash, RoleAdministrator, MemberPending); err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO mfa_enrollments
		(token_hash, admin_id, wrapped_key, nonces, ciphertext, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)`, enrollmentTokenHash, memberID, wrappedKey, nonces, ciphertext, expiresAt); err != nil {
		return err
	}
	if err := s.appendAuditTx(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) MFAEnrollmentByToken(ctx context.Context, tokenHash string, now time.Time) (*MFAEnrollment, error) {
	row := s.pool.QueryRow(ctx, `SELECT admin_id, token_hash, wrapped_key, nonces, ciphertext,
		COALESCE(confirmation_hash, ''), expires_at, verified_at, confirmed_at
		FROM mfa_enrollments WHERE token_hash = $1 AND expires_at > $2`, tokenHash, now)
	var enrollment MFAEnrollment
	if err := row.Scan(&enrollment.MemberID, &enrollment.TokenHash, &enrollment.WrappedKey,
		&enrollment.Nonces, &enrollment.Ciphertext, &enrollment.ConfirmationHash,
		&enrollment.ExpiresAt, &enrollment.VerifiedAt, &enrollment.ConfirmedAt); err != nil {
		return nil, mapNoRows(err)
	}
	return &enrollment, nil
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
	var memberID string
	err = tx.QueryRow(ctx, `SELECT admin_id FROM mfa_enrollments
		WHERE token_hash = $1 AND expires_at > $2 AND verified_at IS NULL FOR UPDATE`, tokenHash, now).Scan(&memberID)
	if err != nil {
		return mapNoRows(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE mfa_enrollments SET verified_at = $2, confirmation_hash = $3 WHERE token_hash = $1`, tokenHash, now, confirmationHash); err != nil {
		return err
	}
	for _, hash := range recoveryCodeHashes {
		if _, err := tx.Exec(ctx, `INSERT INTO recovery_codes (admin_id, code_hash) VALUES ($1, $2)`, memberID, hash); err != nil {
			return err
		}
	}
	if err := s.appendAuditTx(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConfirmMFAEnrollment activates the pending member only after the caller has
// explicitly acknowledged the one-time Recovery Code display.
func (s *Store) ConfirmMFAEnrollment(ctx context.Context, confirmationHash string, now time.Time, audit AuditEvent) (*Member, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var memberID string
	err = tx.QueryRow(ctx, `SELECT admin_id FROM mfa_enrollments
		WHERE confirmation_hash = $1 AND expires_at > $2 AND verified_at IS NOT NULL AND confirmed_at IS NULL FOR UPDATE`, confirmationHash, now).Scan(&memberID)
	if err != nil {
		return nil, mapNoRows(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE mfa_enrollments SET confirmed_at = $2 WHERE confirmation_hash = $1`, confirmationHash, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE admins SET status = $2, activated_at = $3 WHERE id = $1 AND status = $4`, memberID, MemberActive, now, MemberPending); err != nil {
		return nil, err
	}
	if err := s.appendAuditTx(ctx, tx, audit); err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, `SELECT id, username, password_hash, role, status, created_at,
		activated_at, deactivated_at, last_totp_counter FROM admins WHERE id = $1`, memberID)
	member, err := scanMember(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return member, nil
}

func (s *Store) PendingMFAEnrollment(ctx context.Context) (bool, error) {
	var pending bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM admins WHERE status = $1)`, MemberPending).Scan(&pending)
	return pending, err
}

// ResumeMFAEnrollment validates a pending member's password in the app layer
// then swaps the browser-facing token for a fresh one without rotating the
// already enrolled TOTP secret.
func (s *Store) ResumeMFAEnrollment(ctx context.Context, memberID, oldTokenHash, newTokenHash string, expiresAt time.Time) error {
	tag, err := s.pool.Exec(ctx, `UPDATE mfa_enrollments SET token_hash = $3, expires_at = $4
		WHERE admin_id = $1 AND token_hash = $2 AND verified_at IS NULL AND confirmed_at IS NULL`, memberID, oldTokenHash, newTokenHash, expiresAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

// ConsumeRecoveryCode atomically burns one code. Callers must normalize and
// hash it before invoking this method.
func (s *Store) ConsumeRecoveryCode(ctx context.Context, memberID, codeHash string, now time.Time) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE recovery_codes SET used_at = $3
		WHERE admin_id = $1 AND code_hash = $2 AND used_at IS NULL`, memberID, codeHash, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) ReplaceRecoveryCodes(ctx context.Context, memberID string, hashes []string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM recovery_codes WHERE admin_id = $1`, memberID); err != nil {
		return err
	}
	for _, hash := range hashes {
		if _, err := tx.Exec(ctx, `INSERT INTO recovery_codes (admin_id, code_hash) VALUES ($1, $2)`, memberID, hash); err != nil {
			return err
		}
	}
	if err := s.appendAuditTx(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UseTOTP(ctx context.Context, memberID string, counter int64) (bool, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE admins SET last_totp_counter = $2
		WHERE id = $1 AND (last_totp_counter IS NULL OR last_totp_counter < $2)`, memberID, counter)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) TOTPEnrollmentForMember(ctx context.Context, memberID string) (*MFAEnrollment, error) {
	row := s.pool.QueryRow(ctx, `SELECT admin_id, token_hash, wrapped_key, nonces, ciphertext,
		COALESCE(confirmation_hash, ''), expires_at, verified_at, confirmed_at
		FROM mfa_enrollments WHERE admin_id = $1`, memberID)
	var enrollment MFAEnrollment
	if err := row.Scan(&enrollment.MemberID, &enrollment.TokenHash, &enrollment.WrappedKey,
		&enrollment.Nonces, &enrollment.Ciphertext, &enrollment.ConfirmationHash,
		&enrollment.ExpiresAt, &enrollment.VerifiedAt, &enrollment.ConfirmedAt); err != nil {
		return nil, mapNoRows(err)
	}
	return &enrollment, nil
}

func (s *Store) CreateBoundedSession(ctx context.Context, idHash, memberID, csrfToken string,
	absoluteExpiresAt, idleExpiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions
		(id_hash, admin_id, csrf_token, expires_at, last_activity_at, idle_expires_at)
		VALUES ($1, $2, $3, $4, now(), $5)`, idHash, memberID, csrfToken, absoluteExpiresAt, idleExpiresAt)
	return err
}

func (s *Store) TouchSessionActivity(ctx context.Context, idHash string, now, idleExpiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET last_activity_at = $2,
		idle_expires_at = LEAST(expires_at, $3) WHERE id_hash = $1`, idHash, now, idleExpiresAt)
	return err
}

func (s *Store) DeleteMemberSessions(ctx context.Context, memberID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE admin_id = $1`, memberID)
	return err
}

func (s *Store) GrantStepUp(ctx context.Context, sessionIDHash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO step_up_grants (session_id_hash, expires_at)
		VALUES ($1, $2) ON CONFLICT (session_id_hash) DO UPDATE SET granted_at = now(), expires_at = EXCLUDED.expires_at`, sessionIDHash, expiresAt)
	return err
}

func (s *Store) HasStepUp(ctx context.Context, sessionIDHash string, now time.Time) (bool, error) {
	var has bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM step_up_grants WHERE session_id_hash = $1 AND expires_at > $2)`, sessionIDHash, now).Scan(&has)
	return has, err
}

func (s *Store) RevokeStepUp(ctx context.Context, sessionIDHash string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM step_up_grants WHERE session_id_hash = $1`, sessionIDHash)
	return err
}

func (s *Store) appendAuditTx(ctx context.Context, tx pgx.Tx, event AuditEvent) error {
	if event.Actor == "" {
		event.Actor = "system"
	}
	_, err := tx.Exec(ctx, `INSERT INTO audit_events (actor, action, resource, result, correlation_id)
		VALUES ($1, $2, $3, $4, $5)`, event.Actor, event.Action, event.Resource, event.Result, event.CorrelationID)
	return err
}

func (s *Store) DeactivateMember(ctx context.Context, memberID string, audit AuditEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var role, status string
	if err := tx.QueryRow(ctx, `SELECT role, status FROM admins WHERE id = $1 FOR UPDATE`, memberID).Scan(&role, &status); err != nil {
		return mapNoRows(err)
	}
	if status == MemberDeactivated {
		return ErrConflict
	}
	if role == RoleAdministrator {
		var remaining int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM admins WHERE role = $1 AND status = $2 AND id <> $3`, RoleAdministrator, MemberActive, memberID).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			return fmt.Errorf("%w: last active administrator", ErrConflict)
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE admins SET status = $2, deactivated_at = now() WHERE id = $1`, memberID, MemberDeactivated); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE admin_id = $1`, memberID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE member_invitations SET revoked_at = now() WHERE admin_id = $1 AND used_at IS NULL AND revoked_at IS NULL`, memberID); err != nil {
		return err
	}
	if err := s.appendAuditTx(ctx, tx, audit); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
