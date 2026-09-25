package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/inbound"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

// AdminGroup is the Cognito group whose members can use the admin page.
const AdminGroup = "admin"

type adminAuthService struct{ provider outbound.AdminAuthProvider }

func NewAdminAuthService(provider outbound.AdminAuthProvider) inbound.AdminAuthService {
	return &adminAuthService{provider: provider}
}

func (s *adminAuthService) SignIn(ctx context.Context, email, password string) (domain.AdminSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return domain.AdminSession{}, domain.ErrInvalidCredentials
	}
	session, err := s.provider.SignIn(ctx, email, password)
	if err != nil {
		return domain.AdminSession{}, fmt.Errorf("%w: %v", domain.ErrInvalidCredentials, err)
	}
	return session, nil
}

func (s *adminAuthService) CompleteNewPassword(ctx context.Context, email, session, newPassword string) (domain.AdminSession, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || session == "" || len(newPassword) < 12 {
		return domain.AdminSession{}, domain.ErrInvalidCredentials
	}
	result, err := s.provider.RespondNewPassword(ctx, email, session, newPassword)
	if err != nil {
		return domain.AdminSession{}, fmt.Errorf("%w: %v", domain.ErrInvalidCredentials, err)
	}
	return result, nil
}

func (s *adminAuthService) Refresh(ctx context.Context, refreshToken string) (domain.AdminSession, error) {
	session, err := s.provider.Refresh(ctx, refreshToken)
	if err != nil {
		return domain.AdminSession{}, fmt.Errorf("%w: %v", domain.ErrSessionExpired, err)
	}
	return session, nil
}

func (s *adminAuthService) Authenticate(ctx context.Context, accessToken string) (domain.Admin, error) {
	admin, groups, err := s.provider.Verify(ctx, accessToken)
	if err != nil {
		return domain.Admin{}, fmt.Errorf("%w: %v", domain.ErrUnauthorized, err)
	}
	if !slices.Contains(groups, AdminGroup) {
		return domain.Admin{}, domain.ErrForbidden
	}
	return admin, nil
}
