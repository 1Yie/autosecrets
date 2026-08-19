package identity

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
	"autosecrets.dev/core/internal/middleware"
	"github.com/google/uuid"
)

const (
	mfaEnrollmentTTL   = 30 * time.Minute
	loginChallengeTTL  = 5 * time.Minute
	oidcTransactionTTL = 5 * time.Minute
	stepUpTTL          = 10 * time.Minute
	recoveryCodeCount  = 10
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{2,64}$`)

// Domain errors. The handler layer maps these (plus database.ErrNotFound,
// database.ErrConflict, database.ErrDuplicate) onto HTTP status codes and the
// machine-readable error envelope.
var (
	ErrInvalidCredentials    = errors.New("identity: invalid credentials")
	ErrSecondFactorRequired  = errors.New("identity: second factor required")
	ErrAlreadyEnrolled       = errors.New("identity: MFA already enrolled")
	ErrEnrollmentNotPending  = errors.New("identity: MFA enrollment is no longer pending")
	ErrChallengeExpired      = errors.New("identity: login challenge expired")
	ErrPasswordLoginDisabled = errors.New("identity: password login disabled")
	ErrExternalLoginRequired = errors.New("identity: external login required")
	ErrLastExternalLogin     = errors.New("identity: last external login")
)

// Service carries the identity business rules. It holds no HTTP concerns:
// request parsing and status codes live in the handler, persistence in the
// repository, and cross-domain Audit recording behind a narrow interface.
type Service struct {
	repo       Repository
	audit      AuditRecorder
	seal       SecretCipher
	now        func() time.Time
	oidcReady  bool
	oauthReady bool
}

// SecretCipher is the narrow seal/open seam the identity domain needs to
// protect TOTP secrets at rest. *crypto.MasterKey satisfies it directly.
type SecretCipher interface {
	Seal(value []byte) (wrappedKey, nonces, ciphertext []byte, err error)
	Open(wrappedKey, nonces, ciphertext []byte) ([]byte, error)
}

func NewService(repo Repository, audit AuditRecorder, seal SecretCipher, now func() time.Time) *Service {
	return &Service{repo: repo, audit: audit, seal: seal, now: now}
}

func (s *Service) SetOIDCReady(ready bool) { s.oidcReady = ready }

func (s *Service) SetOAuthReady(ready bool) { s.oauthReady = ready }

func (s *Service) PasswordLoginState(ctx context.Context) (enabled, available bool, err error) {
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return false, false, err
	}
	available, err = s.passwordLoginAvailable(ctx, organization)
	return organization.PasswordLoginEnabled, available, err
}

func (s *Service) externalLoginAvailable(ctx context.Context) (bool, error) {
	if s.oidcReady {
		if _, err := s.repo.ExternalIdentityBinding(ctx); err == nil {
			return true, nil
		} else if !errors.Is(err, database.ErrNotFound) {
			return false, err
		}
	}
	if s.oauthReady {
		if _, err := s.repo.OAuthIdentityBinding(ctx); err == nil {
			return true, nil
		} else if !errors.Is(err, database.ErrNotFound) {
			return false, err
		}
	}
	return false, nil
}

func (s *Service) passwordLoginAvailable(ctx context.Context, organization *database.Organization) (bool, error) {
	if organization.PasswordLoginEnabled {
		return true, nil
	}
	external, err := s.externalLoginAvailable(ctx)
	if err != nil {
		return false, err
	}
	return !external, nil
}

func (s *Service) rejectLastExternalUnbind(ctx context.Context, remainingReady bool, remaining func(context.Context) (*database.ExternalIdentityBinding, error)) error {
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return err
	}
	if organization.PasswordLoginEnabled {
		return nil
	}
	if remainingReady {
		if _, err := remaining(ctx); err == nil {
			return nil
		} else if !errors.Is(err, database.ErrNotFound) {
			return err
		}
	}
	return ErrLastExternalLogin
}

// --- correlation id via context (set by the handler layer) -----------------

type correlationIDKey struct{}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey{}, id)
}

func CorrelationID(ctx context.Context) string {
	id, _ := ctx.Value(correlationIDKey{}).(string)
	return id
}

// --- results --------------------------------------------------------------

type BootstrapOutput struct {
	ID       string
	Username string
	Status   string
	Session  *Session
}

type OIDCStart struct {
	State    string
	Nonce    string
	Verifier string
}

type VerifyMFAEnrollmentOutput struct {
	ConfirmationToken string
	RecoveryCodes     []string
}

type TOTPEnrollmentOutput struct {
	Username        string
	EnrollmentToken string
	TOTPURI         string
}

type LoginOutput struct {
	Session        *Session
	ChallengeToken string
}

// Session is the issued-session material returned to the handler, which owns
// the Set-Cookie and JSON serialization.
type Session struct {
	ID        string
	CSRFToken string
	Username  string
	MemberID  string
	Role      string
	ExpiresAt time.Time
}

// --- use cases ------------------------------------------------------------

func (s *Service) Bootstrap(ctx context.Context, code, organizationName, username, password string) (*BootstrapOutput, error) {
	if !validOrganizationName(organizationName) || !usernameRe.MatchString(username) || !crypto.PasswordValid(password) {
		s.auditEvent(ctx, "bootstrap", "administrator.bootstrapped", "", "denied")
		return nil, ErrInvalidCredentials
	}
	passwordHash, err := crypto.HashPassword(password)
	if err != nil {
		s.auditEvent(ctx, "bootstrap", "administrator.bootstrapped", "", "denied")
		return nil, err
	}
	memberID := uuid.NewString()
	now := s.now()
	err = s.repo.BootstrapAdministrator(ctx, crypto.HashToken(code), strings.TrimSpace(organizationName),
		memberID, username, passwordHash, now, database.AuditEvent{
			Actor: "bootstrap", Action: "administrator.bootstrapped", Resource: memberID,
			Result: "active", CorrelationID: CorrelationID(ctx),
		})
	if err != nil {
		s.auditEvent(ctx, "bootstrap", "administrator.bootstrapped", "", "denied")
		return nil, err
	}
	member := &database.Member{ID: memberID, Username: username, Role: database.RoleAdministrator, Status: database.MemberActive}
	session, err := s.issueSession(ctx, member, "local")
	if err != nil {
		return nil, err
	}
	return &BootstrapOutput{ID: memberID, Username: username, Status: "active", Session: session}, nil
}

func (s *Service) VerifyMFAEnrollment(ctx context.Context, enrollmentToken, totpCode string) (*VerifyMFAEnrollmentOutput, error) {
	now := s.now()
	enrollment, err := s.repo.MFAEnrollmentByToken(ctx, crypto.HashToken(enrollmentToken), now)
	if err != nil {
		s.auditEvent(ctx, "authentication", "administrator.totp_enabled", "", "denied")
		return nil, err
	}
	secret, err := s.seal.Open(enrollment.WrappedKey, enrollment.Nonces, enrollment.Ciphertext)
	if err != nil || !crypto.VerifyTOTP(string(secret), totpCode, now) {
		s.auditEvent(ctx, "administrator:"+enrollment.MemberID, "administrator.totp_enabled", enrollment.MemberID, "denied")
		return nil, ErrInvalidCredentials
	}
	recoveryCodes, err := crypto.NewRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, err
	}
	hashes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
		hashes[i] = crypto.HashToken(crypto.NormalizeRecoveryCode(code))
	}
	confirmationToken, err := crypto.NewSecret(192)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CompleteMFAEnrollment(ctx, crypto.HashToken(enrollmentToken), crypto.HashToken(confirmationToken), hashes, now, database.AuditEvent{
		Actor: "member:" + enrollment.MemberID, Action: "member.mfa_verified", Resource: enrollment.MemberID,
		Result: "pending_recovery_confirmation", CorrelationID: CorrelationID(ctx),
	}); err != nil {
		return nil, ErrEnrollmentNotPending
	}
	return &VerifyMFAEnrollmentOutput{ConfirmationToken: confirmationToken, RecoveryCodes: recoveryCodes}, nil
}

func (s *Service) ConfirmMFAEnrollment(ctx context.Context, memberID, currentSessionHash, confirmationToken string) (*database.Member, error) {
	member, err := s.repo.ConfirmMFAEnrollment(ctx, crypto.HashToken(confirmationToken), currentSessionHash, s.now(), database.AuditEvent{
		Actor: "administrator:" + memberID, Action: "administrator.totp_enabled", Resource: memberID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.totp_enabled", memberID, "denied")
		return nil, ErrEnrollmentNotPending
	}
	return member, nil
}

func (s *Service) Login(ctx context.Context, username, password, sourceHash string) (*LoginOutput, error) {
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return nil, err
	}
	allowed, err := s.passwordLoginAvailable(ctx, organization)
	if err != nil {
		return nil, err
	}
	if !allowed {
		s.auditEvent(ctx, "authentication", "auth.login", "", "password_login_disabled")
		return nil, ErrPasswordLoginDisabled
	}
	member, err := s.repo.MemberByUsername(ctx, username)
	if err != nil || member.Status != database.MemberActive {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK {
		s.auditEvent(ctx, "member:"+member.Username, "auth.login", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	if !organization.TOTPLoginRequired {
		session, err := s.issueSession(ctx, member, "local")
		if err == nil {
			s.auditEvent(ctx, "member:"+member.Username, "auth.login", member.ID, "local")
		}
		return &LoginOutput{Session: session}, err
	}
	challenge, err := crypto.NewSecret(192)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateLoginChallenge(ctx, crypto.HashToken(challenge), member.ID, sourceHash, s.now().Add(loginChallengeTTL)); err != nil {
		return nil, err
	}
	return &LoginOutput{ChallengeToken: challenge}, nil
}

func (s *Service) CompleteLogin(ctx context.Context, challengeToken, sourceHash, totpCode, recoveryCode string) (*Session, error) {
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return nil, err
	}
	allowed, err := s.passwordLoginAvailable(ctx, organization)
	if err != nil {
		return nil, err
	}
	if !allowed {
		s.auditEvent(ctx, "authentication", "auth.second_factor", "", "password_login_disabled")
		return nil, ErrPasswordLoginDisabled
	}
	memberID, err := s.repo.ConsumeLoginChallenge(ctx, crypto.HashToken(challengeToken), sourceHash, s.now())
	if err != nil {
		s.auditEvent(ctx, "authentication", "auth.second_factor", "", "denied")
		return nil, ErrChallengeExpired
	}
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil || member.Status != database.MemberActive || !s.verifySecondFactor(ctx, member, totpCode, recoveryCode) {
		if member != nil {
			s.auditEvent(ctx, "member:"+member.Username, "auth.second_factor", member.ID, "denied")
			s.auditEvent(ctx, "member:"+member.Username, "auth.login", member.ID, "denied")
		} else {
			s.auditEvent(ctx, "authentication", "auth.second_factor", "", "denied")
			s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		}
		return nil, ErrInvalidCredentials
	}
	session, err := s.issueSession(ctx, member, "local")
	if err == nil {
		factor := "totp"
		if recoveryCode != "" {
			factor = "recovery_code"
		}
		s.auditEvent(ctx, "member:"+member.Username, "auth.second_factor", member.ID, factor)
		s.auditEvent(ctx, "member:"+member.Username, "auth.login", member.ID, "local")
	}
	return session, err
}

func (s *Service) StartTOTPEnrollment(ctx context.Context, memberID, password string) (*TOTPEnrollmentOutput, error) {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil || member.Status != database.MemberActive {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.totp_enabled", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.totp_enabled", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return nil, err
	}
	if organization.TOTPLoginRequired {
		return nil, ErrAlreadyEnrolled
	}
	totpSecret, err := crypto.NewTOTPSecret()
	if err != nil {
		return nil, err
	}
	wrappedKey, nonces, ciphertext, err := s.seal.Seal([]byte(totpSecret))
	if err != nil {
		return nil, err
	}
	enrollmentToken, err := crypto.NewSecret(192)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateEnrollmentForMember(ctx, member.ID, crypto.HashToken(enrollmentToken), wrappedKey, nonces, ciphertext, s.now().Add(mfaEnrollmentTTL)); err != nil {
		return nil, err
	}
	return &TOTPEnrollmentOutput{Username: member.Username, EnrollmentToken: enrollmentToken, TOTPURI: crypto.TOTPURI("AutoSecrets", member.Username, totpSecret)}, nil
}

func (s *Service) RenewSession(ctx context.Context, oldSessionIDHash, memberID, password, totpCode, recoveryCode string) (*Session, error) {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "auth.renew", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, recoveryCode, true) {
		s.auditEvent(ctx, "member:"+member.Username, "auth.renew", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	_ = s.repo.DeleteSession(ctx, oldSessionIDHash)
	session, err := s.issueSession(ctx, member, "local")
	if err == nil {
		s.auditEvent(ctx, "member:"+member.Username, "auth.renew", member.ID, "local")
	}
	return session, err
}

func (s *Service) StepUp(ctx context.Context, sessionIDHash, memberID, password, totpCode, recoveryCode string) (time.Time, error) {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "authentication", "auth.step_up", "", "denied")
		return time.Time{}, ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, recoveryCode, true) {
		_ = s.audit.AppendAudit(ctx, database.AuditEvent{
			Actor: "member:" + member.Username, Action: "auth.step_up", Resource: member.ID,
			Result: "denied", CorrelationID: CorrelationID(ctx),
		})
		return time.Time{}, ErrInvalidCredentials
	}
	now := s.now()
	expiresAt := now.Add(stepUpTTL)
	if err := s.repo.UpsertStepUpGrant(ctx, sessionIDHash, now, expiresAt); err != nil {
		return time.Time{}, err
	}
	_ = s.audit.AppendAudit(ctx, database.AuditEvent{
		Actor: "member:" + member.Username, Action: "auth.step_up", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
	return expiresAt, nil
}

func (s *Service) ChangePassword(ctx context.Context, memberID, currentPassword, newPassword, totpCode string) (*Session, error) {
	if !crypto.PasswordValid(newPassword) {
		s.auditEvent(ctx, "administrator:"+memberID, "member.password_changed", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "member.password_changed", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	ok, err := crypto.VerifyPassword(currentPassword, member.PasswordHash)
	if err != nil || !ok || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "member:"+member.Username, "member.password_changed", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	newHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ChangePassword(ctx, member.ID, newHash, database.AuditEvent{
		Actor: "member:" + member.Username, Action: "member.password_changed", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	}); err != nil {
		return nil, err
	}
	return s.issueSession(ctx, member, "local")
}

func (s *Service) ChangeUsername(ctx context.Context, memberID, username, currentPassword, totpCode string) (*Session, error) {
	if !usernameRe.MatchString(username) {
		s.auditEvent(ctx, "administrator:"+memberID, "member.username_changed", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "member.username_changed", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	if username == member.Username {
		s.auditEvent(ctx, "member:"+member.Username, "member.username_changed", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	ok, err := crypto.VerifyPassword(currentPassword, member.PasswordHash)
	if err != nil || !ok || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "member:"+member.Username, "member.username_changed", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	if err := s.repo.ChangeUsername(ctx, member.ID, username, database.AuditEvent{
		Actor: "member:" + member.Username, Action: "member.username_changed", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	}); err != nil {
		return nil, err
	}
	member.Username = username
	return s.issueSession(ctx, member, "local")
}

func (s *Service) Logout(ctx context.Context, sessionIDHash, memberID, username string) {
	if sessionIDHash != "" {
		_ = s.repo.DeleteSession(ctx, sessionIDHash)
		s.auditEvent(ctx, "member:"+username, "auth.logout", memberID, "ok")
	}
}

func (s *Service) issueSession(ctx context.Context, member *database.Member, authMethod string) (*Session, error) {
	sessionID, err := crypto.NewSecret(256)
	if err != nil {
		return nil, err
	}
	csrf, err := crypto.NewSecret(128)
	if err != nil {
		return nil, err
	}
	now := s.now()
	absoluteExpires := now.Add(middleware.SessionAbsoluteTTL)
	idleExpires := now.Add(middleware.SessionIdleTTL)
	if err := s.repo.CreateBoundedSession(ctx, crypto.HashToken(sessionID), member.ID, csrf, absoluteExpires, idleExpires, authMethod); err != nil {
		return nil, err
	}
	return &Session{
		ID: sessionID, CSRFToken: csrf, Username: member.Username,
		MemberID: member.ID, Role: member.Role, ExpiresAt: absoluteExpires,
	}, nil
}

func (s *Service) policyProofValid(ctx context.Context, member *database.Member, totpCode, recoveryCode string, allowRecovery bool) bool {
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return false
	}
	if !organization.TOTPLoginRequired {
		return true
	}
	if !allowRecovery {
		recoveryCode = ""
	}
	return s.verifySecondFactor(ctx, member, totpCode, recoveryCode)
}

func (s *Service) DisableTOTP(ctx context.Context, memberID, currentSessionHash, password, totpCode string) error {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.totp_disabled", memberID, "denied")
		return ErrInvalidCredentials
	}
	ok, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !ok || !s.verifySecondFactor(ctx, member, totpCode, "") {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.totp_disabled", member.ID, "denied")
		return ErrInvalidCredentials
	}
	return s.repo.DisableTOTP(ctx, memberID, currentSessionHash, database.AuditEvent{
		Actor: "administrator:" + member.Username, Action: "administrator.totp_disabled", Resource: memberID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
}

func (s *Service) SetPasswordLoginEnabled(ctx context.Context, memberID, currentSessionHash, password, totpCode string, enabled bool) error {
	action := "administrator.password_login_enabled"
	if !enabled {
		action = "administrator.password_login_disabled"
	}
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, action, memberID, "denied")
		return ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "administrator:"+member.Username, action, member.ID, "denied")
		return ErrInvalidCredentials
	}
	organization, err := s.repo.Organization(ctx)
	if err != nil {
		return err
	}
	if organization.PasswordLoginEnabled == enabled {
		return nil
	}
	if !enabled {
		usable, err := s.externalLoginAvailable(ctx)
		if err != nil {
			return err
		}
		if !usable {
			s.auditEvent(ctx, "administrator:"+member.Username, action, member.ID, "denied")
			return ErrExternalLoginRequired
		}
	}
	return s.repo.SetPasswordLoginEnabled(ctx, memberID, currentSessionHash, enabled, database.AuditEvent{
		Actor: "administrator:" + member.Username, Action: action, Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
}

func (s *Service) StartOIDCLogin(ctx context.Context, returnTo string) (*OIDCStart, error) {
	binding, err := s.repo.ExternalIdentityBinding(ctx)
	if err != nil || binding == nil {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	return s.createOIDCTransaction(ctx, "login", "", returnTo)
}

func (s *Service) StartOIDCBinding(ctx context.Context, memberID, password, totpCode, returnTo string) (*OIDCStart, error) {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.oidc_bound", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oidc_bound", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	if _, err := s.repo.ExternalIdentityBinding(ctx); err == nil {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oidc_bound", member.ID, "conflict")
		return nil, database.ErrConflict
	} else if !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}
	return s.createOIDCTransaction(ctx, "bind", memberID, returnTo)
}

func (s *Service) createOIDCTransaction(ctx context.Context, purpose, memberID, returnTo string) (*OIDCStart, error) {
	state, err := crypto.NewSecret(192)
	if err != nil {
		return nil, err
	}
	nonce, err := crypto.NewSecret(192)
	if err != nil {
		return nil, err
	}
	verifier, err := crypto.NewSecret(256)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateOIDCTransaction(ctx, crypto.HashToken(state), purpose, memberID, nonce, verifier, returnTo, s.now().Add(oidcTransactionTTL)); err != nil {
		return nil, err
	}
	return &OIDCStart{State: state, Nonce: nonce, Verifier: verifier}, nil
}

func (s *Service) ConsumeOIDCTransaction(ctx context.Context, state string) (*database.OIDCTransaction, error) {
	transaction, err := s.repo.ConsumeOIDCTransaction(ctx, crypto.HashToken(state), s.now())
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return transaction, nil
}

func (s *Service) CompleteOIDCLogin(ctx context.Context, transaction *database.OIDCTransaction, identity *OIDCIdentity) (*Session, error) {
	if transaction == nil || transaction.Purpose != "login" || identity == nil {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	binding, err := s.repo.ExternalIdentityBinding(ctx)
	if err != nil || binding.Issuer != identity.Issuer || binding.Subject != identity.Subject {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	member, err := s.repo.MemberByID(ctx, binding.MemberID)
	if err != nil || member.Status != database.MemberActive {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	session, err := s.issueSession(ctx, member, "oidc")
	if err == nil {
		s.auditEvent(ctx, "member:"+member.Username, "auth.login", member.ID, "oidc")
	}
	return session, err
}

func (s *Service) CompleteOIDCBinding(ctx context.Context, transaction *database.OIDCTransaction, identity *OIDCIdentity, currentSessionHash string) error {
	if transaction == nil || transaction.Purpose != "bind" || transaction.MemberID == "" || identity == nil {
		s.auditEvent(ctx, "authentication", "administrator.oidc_bound", "", "denied")
		return ErrInvalidCredentials
	}
	member, err := s.repo.MemberByID(ctx, transaction.MemberID)
	if err != nil || member.Status != database.MemberActive {
		s.auditEvent(ctx, "administrator:"+transaction.MemberID, "administrator.oidc_bound", transaction.MemberID, "denied")
		return ErrInvalidCredentials
	}
	return s.repo.BindExternalIdentity(ctx, database.ExternalIdentityBinding{
		MemberID: transaction.MemberID, Issuer: identity.Issuer,
		Subject: identity.Subject, DisplayName: identity.DisplayName,
	}, currentSessionHash, database.AuditEvent{
		Actor: "administrator:" + member.Username, Action: "administrator.oidc_bound", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
}

func (s *Service) StartOAuthLogin(ctx context.Context, returnTo string) (*OIDCStart, error) {
	binding, err := s.repo.OAuthIdentityBinding(ctx)
	if err != nil || binding == nil {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	return s.createOIDCTransaction(ctx, "login", "", returnTo)
}

func (s *Service) StartOAuthBinding(ctx context.Context, memberID, password, totpCode, returnTo string) (*OIDCStart, error) {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.oauth_bound", memberID, "denied")
		return nil, ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oauth_bound", member.ID, "denied")
		return nil, ErrInvalidCredentials
	}
	if _, err := s.repo.OAuthIdentityBinding(ctx); err == nil {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oauth_bound", member.ID, "conflict")
		return nil, database.ErrConflict
	} else if !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}
	return s.createOIDCTransaction(ctx, "bind", memberID, returnTo)
}

func (s *Service) CompleteOAuthLogin(ctx context.Context, transaction *database.OIDCTransaction, identity *OIDCIdentity) (*Session, error) {
	if transaction == nil || transaction.Purpose != "login" || identity == nil {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	binding, err := s.repo.OAuthIdentityBinding(ctx)
	if err != nil || binding.Issuer != identity.Issuer || binding.Subject != identity.Subject {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	member, err := s.repo.MemberByID(ctx, binding.MemberID)
	if err != nil || member.Status != database.MemberActive {
		s.auditEvent(ctx, "authentication", "auth.login", "", "denied")
		return nil, ErrInvalidCredentials
	}
	session, err := s.issueSession(ctx, member, "oauth")
	if err == nil {
		s.auditEvent(ctx, "member:"+member.Username, "auth.login", member.ID, "oauth")
	}
	return session, err
}

func (s *Service) CompleteOAuthBinding(ctx context.Context, transaction *database.OIDCTransaction, identity *OIDCIdentity, currentSessionHash string) error {
	if transaction == nil || transaction.Purpose != "bind" || transaction.MemberID == "" || identity == nil {
		s.auditEvent(ctx, "authentication", "administrator.oauth_bound", "", "denied")
		return ErrInvalidCredentials
	}
	member, err := s.repo.MemberByID(ctx, transaction.MemberID)
	if err != nil || member.Status != database.MemberActive {
		s.auditEvent(ctx, "administrator:"+transaction.MemberID, "administrator.oauth_bound", transaction.MemberID, "denied")
		return ErrInvalidCredentials
	}
	return s.repo.BindOAuthIdentity(ctx, database.ExternalIdentityBinding{
		MemberID: transaction.MemberID, Issuer: identity.Issuer,
		Subject: identity.Subject, DisplayName: identity.DisplayName,
	}, currentSessionHash, database.AuditEvent{
		Actor: "administrator:" + member.Username, Action: "administrator.oauth_bound", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
}

func (s *Service) UnbindOAuth(ctx context.Context, memberID, currentSessionHash, password, totpCode string) error {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.oauth_unbound", memberID, "denied")
		return ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oauth_unbound", member.ID, "denied")
		return ErrInvalidCredentials
	}
	if _, err := s.repo.OAuthIdentityBinding(ctx); err != nil {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oauth_unbound", member.ID, "denied")
		return database.ErrNotFound
	}
	if err := s.rejectLastExternalUnbind(ctx, s.oidcReady, s.repo.ExternalIdentityBinding); err != nil {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oauth_unbound", member.ID, "denied")
		return err
	}
	return s.repo.UnbindOAuthIdentity(ctx, memberID, currentSessionHash, database.AuditEvent{
		Actor: "administrator:" + member.Username, Action: "administrator.oauth_unbound", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
}

func (s *Service) UnbindOIDC(ctx context.Context, memberID, currentSessionHash, password, totpCode string) error {
	member, err := s.repo.MemberByID(ctx, memberID)
	if err != nil {
		s.auditEvent(ctx, "administrator:"+memberID, "administrator.oidc_unbound", memberID, "denied")
		return ErrInvalidCredentials
	}
	passwordOK, err := crypto.VerifyPassword(password, member.PasswordHash)
	if err != nil || !passwordOK || !s.policyProofValid(ctx, member, totpCode, "", false) {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oidc_unbound", member.ID, "denied")
		return ErrInvalidCredentials
	}
	if _, err := s.repo.ExternalIdentityBinding(ctx); err != nil {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oidc_unbound", member.ID, "denied")
		return database.ErrNotFound
	}
	if err := s.rejectLastExternalUnbind(ctx, s.oauthReady, s.repo.OAuthIdentityBinding); err != nil {
		s.auditEvent(ctx, "administrator:"+member.Username, "administrator.oidc_unbound", member.ID, "denied")
		return err
	}
	return s.repo.UnbindExternalIdentity(ctx, memberID, currentSessionHash, database.AuditEvent{
		Actor: "administrator:" + member.Username, Action: "administrator.oidc_unbound", Resource: member.ID,
		Result: "ok", CorrelationID: CorrelationID(ctx),
	})
}

func (s *Service) verifySecondFactor(ctx context.Context, member *database.Member, totpCode, recoveryCode string) bool {
	if recoveryCode != "" {
		used, err := s.repo.ConsumeRecoveryCode(ctx, member.ID, crypto.HashToken(crypto.NormalizeRecoveryCode(recoveryCode)), s.now())
		return err == nil && used
	}
	enrollment, err := s.repo.TOTPEnrollmentForMember(ctx, member.ID)
	if err != nil || enrollment.VerifiedAt == nil || enrollment.ConfirmedAt == nil {
		return false
	}
	secret, err := s.seal.Open(enrollment.WrappedKey, enrollment.Nonces, enrollment.Ciphertext)
	if err != nil {
		return false
	}
	counter, valid := crypto.TOTPMatchingCounter(string(secret), totpCode, s.now())
	if !valid {
		return false
	}
	used, err := s.repo.UseTOTP(ctx, member.ID, counter)
	return err == nil && used
}

func (s *Service) AuditOIDCCallbackDenied(ctx context.Context, purpose, memberID string) {
	s.auditExternalCallbackDenied(ctx, purpose, memberID, "administrator.oidc_bound")
}

func (s *Service) AuditOAuthCallbackDenied(ctx context.Context, purpose, memberID string) {
	s.auditExternalCallbackDenied(ctx, purpose, memberID, "administrator.oauth_bound")
}

func (s *Service) auditExternalCallbackDenied(ctx context.Context, purpose, memberID, bindAction string) {
	action := "auth.login"
	actor := "authentication"
	if purpose == "bind" {
		action = bindAction
		actor = "administrator:" + memberID
	}
	s.auditEvent(ctx, actor, action, memberID, "denied")
}

func (s *Service) auditEvent(ctx context.Context, actor, action, resource, result string) {
	_ = s.audit.AppendAudit(ctx, database.AuditEvent{
		Actor: actor, Action: action, Resource: resource, Result: result, CorrelationID: CorrelationID(ctx),
	})
}

func validOrganizationName(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) > 0 && len(value) <= 128
}
