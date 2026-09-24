package service

import (
	"context"
	"fmt"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type accountService struct {
	auth  outbound.AuthProvider
	users outbound.UserRepository
}

func NewAccountService(auth outbound.AuthProvider, users outbound.UserRepository) inbound.AccountService {
	return &accountService{auth: auth, users: users}
}

// DeleteAccount removes the database rows first: if Cognito then fails, the
// user can still sign in and retry, and nothing about them is left behind.
func (s *accountService) DeleteAccount(ctx context.Context, p domain.Principal) error {
	if err := s.users.Delete(ctx, p.UserID); err != nil {
		return fmt.Errorf("delete user rows: %w", err)
	}
	if err := s.auth.Delete(ctx, p.Username); err != nil {
		return fmt.Errorf("delete auth user: %w", err)
	}
	return nil
}
