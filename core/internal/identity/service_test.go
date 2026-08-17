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
	organization     func(context.Context) (*database.Organization, error)
	hasConfirmedMFA  func(context.Context, string) (bool, error)
	consumeRecovery  func(context.Context, string, string, time.Time) (bool, error)
	createSession    func(context.Context, string, string, string, time.Time, time.Time, string) error
	createChallenge  func(context.Context, string, string, string, time.Time) error
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

func (f *fakeRepo) ExternalIdentityBinding(context.Context) (*database.ExternalIdentityBinding, error) {
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

func (f *fakeRepo) UnbindExternalIdentity(context.Context, string, string, database.AuditEvent) error {
	return nil
}

func (f *fakeRepo) OAuthIdentityBinding(context.Context) (*database.ExternalIdentityBinding, error) {
	return nil, database.ErrNotFound
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
