package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/reangeline/missale-backend/internal/core/domain"
)

type fakeAccountAuth struct{ deleted []string }

func (f *fakeAccountAuth) SignIn(context.Context, domain.AppleIdentity) (string, domain.Session, error) {
	return "", domain.Session{}, nil
}
func (f *fakeAccountAuth) Refresh(context.Context, string) (domain.Session, error) {
	return domain.Session{}, nil
}
func (f *fakeAccountAuth) Verify(context.Context, string) (domain.Principal, error) {
	return domain.Principal{}, nil
}
func (f *fakeAccountAuth) Delete(_ context.Context, username string) error {
	f.deleted = append(f.deleted, username)
	return nil
}

type fakeAccountUsers struct{ deleted []string }

func (f *fakeAccountUsers) Save(context.Context, domain.User) error { return nil }
func (f *fakeAccountUsers) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeRevoker struct {
	revoked []string
	err     error
}

func (f *fakeRevoker) Revoke(_ context.Context, code string) error {
	f.revoked = append(f.revoked, code)
	return f.err
}

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestDeleteAccountRevokesWhenACodeIsSent(t *testing.T) {
	auth, users, revoker := &fakeAccountAuth{}, &fakeAccountUsers{}, &fakeRevoker{}
	s := NewAccountService(auth, users, revoker, testLogger())

	if err := s.DeleteAccount(context.Background(), domain.Principal{UserID: "u1", Username: "apple_u1"}, "fresh-code"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if len(revoker.revoked) != 1 || revoker.revoked[0] != "fresh-code" {
		t.Fatalf("revoker.revoked = %v", revoker.revoked)
	}
	if len(users.deleted) != 1 || len(auth.deleted) != 1 {
		t.Fatalf("account not fully deleted: users=%v auth=%v", users.deleted, auth.deleted)
	}
}

func TestDeleteAccountStillDeletesWhenRevocationFails(t *testing.T) {
	auth, users := &fakeAccountAuth{}, &fakeAccountUsers{}
	revoker := &fakeRevoker{err: errors.New("apple is down")}
	s := NewAccountService(auth, users, revoker, testLogger())

	err := s.DeleteAccount(context.Background(), domain.Principal{UserID: "u1", Username: "apple_u1"}, "fresh-code")
	if err != nil {
		t.Fatalf("DeleteAccount must succeed even when Apple revocation fails, got: %v", err)
	}
	if len(users.deleted) != 1 || users.deleted[0] != "u1" {
		t.Fatalf("user rows not deleted: %v", users.deleted)
	}
	if len(auth.deleted) != 1 || auth.deleted[0] != "apple_u1" {
		t.Fatalf("cognito user not deleted: %v", auth.deleted)
	}
}

func TestDeleteAccountWithoutACodeDoesNotCallTheRevoker(t *testing.T) {
	auth, users, revoker := &fakeAccountAuth{}, &fakeAccountUsers{}, &fakeRevoker{}
	s := NewAccountService(auth, users, revoker, testLogger())

	if err := s.DeleteAccount(context.Background(), domain.Principal{UserID: "u1", Username: "apple_u1"}, ""); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if len(revoker.revoked) != 0 {
		t.Fatalf("revoker should not run without a code, got %v", revoker.revoked)
	}
}
