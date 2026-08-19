package identity

import (
	"context"
	"testing"
	"time"

	"autosecrets.dev/core/internal/crypto"
	"autosecrets.dev/core/internal/database"
)

// fakeRepo overrides only the methods a given test exercises. It embeds the
// Repository interface so any unexpected call fails fast rather than silently
// returning a zero value.
type fakeRepo struct {
	Repository
	memberByUsername func(context.Context, string) (*database.Member, error)
	memberByID       func(context.Context, string) (*database.Member, error)
	organization     func(context.Context) (*database.Organization, error)
	hasConfirmedMFA  func(context.Context, string) (bool, error)
	consumeRecovery  func(context.Context, string, string, time.Time) (bool, error)
	createSession    func(context.Context, string, string, string, time.Time, time.Time, string) error
	createChallenge  func(context.Context, string, string, string, time.Time) error
	oidcBinding      func(context.Context) (*database.ExternalIdentityBinding, error)
	oauthBinding     func(context.Context) (*database.ExternalIdentityBinding, error)
	setPasswordLogin func(context.Context, string, string, bool, database.AuditEvent) error
	unbindExternal   func(context.Context, string, string, database.AuditEvent) error
}

func (f *fakeRepo) MemberByUsername(ctx context.Context, u string) (*database.Member, error) {
	return f.memberByUsername(ctx, u)
}

func (f *fakeRepo) HasConfirmedMFA(ctx context.Context, id string) (bool, error) {
	return f.hasConfirmedMFA(ctx, id)
}

func (f *fakeRepo) Organization(ctx context.Context) (*database.Organization, error) {
	return f.organization(ctx)
}

func (f *fakeRepo) ConsumeRecoveryCode(ctx context.Context, id, hash string, now time.Time) (bool, error) {
	return f.consumeRecovery(ctx, id, hash, now)
}

func (f *fakeRepo) CreateBoundedSession(ctx context.Context, idHash, memberID, csrf string, abs, idle time.Time, method string) error {
	return f.createSession(ctx, idHash, memberID, csrf, abs, idle, method)
}

func (f *fakeRepo) CreateLoginChallenge(ctx context.Context, tokenHash, memberID, sourceHash string, expires time.Time) error {
	return f.createChallenge(ctx, tokenHash, memberID, sourceHash, expires)
}

func (f *fakeRepo) MemberByID(ctx context.Context, id string) (*database.Member, error) {
	return f.memberByID(ctx, id)
}

func (f *fakeRepo) ExternalIdentityBinding(ctx context.Context) (*database.ExternalIdentityBinding, error) {
	if f.oidcBinding != nil {
		return f.oidcBinding(ctx)
	}
	return nil, database.ErrNotFound
}

func (f *fakeRepo) CreateOIDCTransaction(context.Context, string, string, string, string, string, string, time.Time) error {
	return nil
}

func (f *fakeRepo) ConsumeOIDCTransaction(context.Context, string, time.Time) (*database.OIDCTransaction, error) {
	return nil, database.ErrNotFound
}

func (f *fakeRepo) BindExternalIdentity(context.Context, database.ExternalIdentityBinding, string, database.AuditEvent) error {
	return nil
}

func (f *fakeRepo) UnbindExternalIdentity(ctx context.Context, memberID, sessionHash string, audit database.AuditEvent) error {
	if f.unbindExternal != nil {
		return f.unbindExternal(ctx, memberID, sessionHash, audit)
	}
	return nil
}

func (f *fakeRepo) OAuthIdentityBinding(ctx context.Context) (*database.ExternalIdentityBinding, error) {
	if f.oauthBinding != nil {
		return f.oauthBinding(ctx)
	}
	return nil, database.ErrNotFound
}

func (f *fakeRepo) SetPasswordLoginEnabled(ctx context.Context, memberID, sessionHash string, enabled bool, audit database.AuditEvent) error {
	return f.setPasswordLogin(ctx, memberID, sessionHash, enabled, audit)
}

func (f *fakeRepo) BindOAuthIdentity(context.Context, database.ExternalIdentityBinding, string, database.AuditEvent) error {
	return nil
}

func (f *fakeRepo) UnbindOAuthIdentity(context.Context, string, string, database.AuditEvent) error {
	return nil
}

type fakeAudit struct{ events []database.AuditEvent }

func (f *fakeAudit) AppendAudit(ctx context.Context, e database.AuditEvent) error {
	f.events = append(f.events, e)
	return nil
}

func newTestService(repo Repository, audit AuditRecorder) *Service {
	return NewService(repo, audit, nil, func() time.Time { return time.Now() })
}

func TestLoginUnknownUserDenied(t *testing.T) {
	repo := &fakeRepo{
		memberByUsername: func(ctx context.Context, u string) (*database.Member, error) {
			return nil, database.ErrNotFound
		},
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{PasswordLoginEnabled: true}, nil
		},
	}
	audit := &fakeAudit{}
	svc := newTestService(repo, audit)

	_, err := svc.Login(context.Background(), "ghost", "pw", "source")
	if err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Result != "denied" {
		t.Fatalf("want one denied audit event, got %#v", audit.events)
	}
}

func TestLoginWithoutTOTPPolicyIssuesSession(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	var created bool
	repo := &fakeRepo{
		memberByUsername: func(ctx context.Context, u string) (*database.Member, error) {
			return &database.Member{ID: "m1", Username: u, PasswordHash: hash, Status: database.MemberActive}, nil
		},
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{TOTPLoginRequired: false}, nil
		},
		createSession: func(context.Context, string, string, string, time.Time, time.Time, string) error {
			created = true
			return nil
		},
	}
	svc := newTestService(repo, &fakeAudit{})

	out, err := svc.Login(context.Background(), "admin", "correct-horse", "source")
	if err != nil || out.Session == nil || !created {
		t.Fatalf("password-only login did not issue a Session: out=%#v err=%v", out, err)
	}
}

func TestLoginWithTOTPPolicyIssuesChallengeOnly(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	var challenged bool
	repo := &fakeRepo{
		memberByUsername: func(ctx context.Context, u string) (*database.Member, error) {
			return &database.Member{ID: "m1", Username: u, PasswordHash: hash, Role: database.RoleAdministrator, Status: database.MemberActive}, nil
		},
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{TOTPLoginRequired: true}, nil
		},
		createChallenge: func(ctx context.Context, tokenHash, memberID, sourceHash string, expires time.Time) error {
			challenged = tokenHash != "" && memberID == "m1" && sourceHash == "source"
			return nil
		},
	}
	audit := &fakeAudit{}
	svc := newTestService(repo, audit)

	out, err := svc.Login(context.Background(), "admin", "correct-horse", "source")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Session != nil || out.ChallengeToken == "" {
		t.Fatalf("want a challenge without a Session, got %#v", out)
	}
	if !challenged {
		t.Fatal("expected CreateLoginChallenge to be called")
	}
	if len(audit.events) != 0 {
		t.Fatalf("password step must not audit a completed login: %#v", audit.events)
	}
}

func TestLoginDisabledWhenExternalLoginIsUsable(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	repo := &fakeRepo{
		memberByUsername: func(context.Context, string) (*database.Member, error) {
			return &database.Member{ID: "m1", Username: "admin", PasswordHash: hash, Status: database.MemberActive}, nil
		},
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{PasswordLoginEnabled: false}, nil
		},
		oauthBinding: func(context.Context) (*database.ExternalIdentityBinding, error) {
			return &database.ExternalIdentityBinding{MemberID: "m1"}, nil
		},
	}
	svc := newTestService(repo, &fakeAudit{})
	svc.SetOAuthReady(true)

	_, err := svc.Login(context.Background(), "admin", "correct-horse", "source")
	if err != ErrPasswordLoginDisabled {
		t.Fatalf("want ErrPasswordLoginDisabled, got %v", err)
	}
}

func TestLoginFailOpenWhenExternalLoginIsUnavailable(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	var created bool
	repo := &fakeRepo{
		memberByUsername: func(context.Context, string) (*database.Member, error) {
			return &database.Member{ID: "m1", Username: "admin", PasswordHash: hash, Status: database.MemberActive}, nil
		},
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{PasswordLoginEnabled: false}, nil
		},
		oauthBinding: func(context.Context) (*database.ExternalIdentityBinding, error) {
			return &database.ExternalIdentityBinding{MemberID: "m1"}, nil
		},
		createSession: func(context.Context, string, string, string, time.Time, time.Time, string) error {
			created = true
			return nil
		},
	}
	svc := newTestService(repo, &fakeAudit{})

	out, err := svc.Login(context.Background(), "admin", "correct-horse", "source")
	if err != nil || out.Session == nil || !created {
		t.Fatalf("fail-open password login: out=%#v err=%v", out, err)
	}
}

func TestDisablePasswordLoginRequiresUsableExternalLogin(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	member := &database.Member{ID: "m1", Username: "admin", PasswordHash: hash, Status: database.MemberActive}
	repo := &fakeRepo{
		memberByID: func(context.Context, string) (*database.Member, error) { return member, nil },
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{PasswordLoginEnabled: true}, nil
		},
	}
	svc := newTestService(repo, &fakeAudit{})

	err := svc.SetPasswordLoginEnabled(context.Background(), "m1", "sess", "correct-horse", "", false)
	if err != ErrExternalLoginRequired {
		t.Fatalf("want ErrExternalLoginRequired, got %v", err)
	}
}

func TestDisablePasswordLoginPersistsWhenExternalLoginIsUsable(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	member := &database.Member{ID: "m1", Username: "admin", PasswordHash: hash, Status: database.MemberActive}
	var stored *bool
	repo := &fakeRepo{
		memberByID: func(context.Context, string) (*database.Member, error) { return member, nil },
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{PasswordLoginEnabled: true}, nil
		},
		oauthBinding: func(context.Context) (*database.ExternalIdentityBinding, error) {
			return &database.ExternalIdentityBinding{MemberID: "m1"}, nil
		},
		setPasswordLogin: func(_ context.Context, _, _ string, enabled bool, _ database.AuditEvent) error {
			stored = &enabled
			return nil
		},
	}
	svc := newTestService(repo, &fakeAudit{})
	svc.SetOAuthReady(true)

	if err := svc.SetPasswordLoginEnabled(context.Background(), "m1", "sess", "correct-horse", "", false); err != nil {
		t.Fatalf("disable password login: %v", err)
	}
	if stored == nil || *stored {
		t.Fatalf("want password login stored as disabled, got %#v", stored)
	}
}

func TestUnbindLastExternalLoginRefusedWhilePasswordLoginDisabled(t *testing.T) {
	hash, _ := crypto.HashPassword("correct-horse")
	member := &database.Member{ID: "m1", Username: "admin", PasswordHash: hash, Status: database.MemberActive}
	repo := &fakeRepo{
		memberByID: func(context.Context, string) (*database.Member, error) { return member, nil },
		organization: func(context.Context) (*database.Organization, error) {
			return &database.Organization{PasswordLoginEnabled: false}, nil
		},
		oauthBinding: func(context.Context) (*database.ExternalIdentityBinding, error) {
			return &database.ExternalIdentityBinding{MemberID: "m1"}, nil
		},
	}
	svc := newTestService(repo, &fakeAudit{})
	svc.SetOAuthReady(true)

	err := svc.UnbindOAuth(context.Background(), "m1", "sess", "correct-horse", "")
	if err != ErrLastExternalLogin {
		t.Fatalf("want ErrLastExternalLogin, got %v", err)
	}
}
