package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type accountService struct {
	auth    outbound.AuthProvider
	users   outbound.UserRepository
	revoker outbound.AppleTokenRevoker
	log     *slog.Logger
}

func NewAccountService(auth outbound.AuthProvider, users outbound.UserRepository, revoker outbound.AppleTokenRevoker, log *slog.Logger) inbound.AccountService {
	return &accountService{auth: auth, users: users, revoker: revoker, log: log}
}

// DeleteAccount removes the database rows first: if Cognito then fails, the
// user can still sign in and retry, and nothing about them is left behind.
//
// When authorizationCode is set, it's a Sign in with Apple code the app just
// got, only to revoke the resulting token here (guideline 5.1.1(v)) — we
// never store an Apple token ourselves. Revocation never blocks deletion: if
// Apple is unreachable, or the code is stale, we still erase our data and
// only log the failure (never the code or a token).
func (s *accountService) DeleteAccount(ctx context.Context, p domain.Principal, authorizationCode string) error {
	if authorizationCode != "" {
		if err := s.revoker.Revoke(ctx, authorizationCode); err != nil {
			s.log.Warn("could not revoke the Apple token; deleting the account anyway", "err", err)
		}
	}
	if err := s.users.Delete(ctx, p.UserID); err != nil {
		return fmt.Errorf("delete user rows: %w", err)
	}
	if err := s.auth.Delete(ctx, p.Username); err != nil {
		return fmt.Errorf("delete auth user: %w", err)
	}
	return nil
}
