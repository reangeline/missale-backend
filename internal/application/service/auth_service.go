package service

import (
	"context"
	"fmt"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type authService struct {
	identity outbound.IdentityVerifier
	auth     outbound.AuthProvider
	users    outbound.UserRepository
}

func NewAuthService(identity outbound.IdentityVerifier, auth outbound.AuthProvider, users outbound.UserRepository) inbound.AuthService {
	return &authService{identity: identity, auth: auth, users: users}
}

func (s *authService) SignInWithApple(ctx context.Context, identityToken string) (domain.Session, error) {
	identity, err := s.identity.Verify(ctx, identityToken)
	if err != nil {
		return domain.Session{}, fmt.Errorf("%w: %v", domain.ErrInvalidAppleToken, err)
	}
	userID, session, err := s.auth.SignIn(ctx, identity)
	if err != nil {
		return domain.Session{}, fmt.Errorf("sign in: %w", err)
	}
	if err := s.users.Save(ctx, domain.User{ID: userID, AppleSub: identity.Sub}); err != nil {
		return domain.Session{}, fmt.Errorf("save user: %w", err)
	}
	return session, nil
}

func (s *authService) RefreshSession(ctx context.Context, refreshToken string) (domain.Session, error) {
	session, err := s.auth.Refresh(ctx, refreshToken)
	if err != nil {
		return domain.Session{}, fmt.Errorf("%w: %v", domain.ErrSessionExpired, err)
	}
	return session, nil
}

func (s *authService) Authenticate(ctx context.Context, accessToken string) (domain.Principal, error) {
	p, err := s.auth.Verify(ctx, accessToken)
	if err != nil {
		return domain.Principal{}, fmt.Errorf("%w: %v", domain.ErrUnauthorized, err)
	}
	return p, nil
}
