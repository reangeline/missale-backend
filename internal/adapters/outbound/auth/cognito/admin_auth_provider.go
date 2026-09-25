package cognito

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/golang-jwt/jwt/v5"

	"github.com/reangeline/missale-backend/internal/adapters/outbound/auth/jwks"
	"github.com/reangeline/missale-backend/internal/core/domain"
	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

// adminAuthProvider signs admins in with email and password through their own
// app client in the same pool, so an app session can never open the admin
// page and vice versa. Admin usernames are their email addresses.
type adminAuthProvider struct {
	client   *cip.Client
	clientID string
	jwks     *jwks.JWKS
	issuer   string
}

func NewAdminAuthProvider(cfg aws.Config, region, poolID, adminClientID string) outbound.AdminAuthProvider {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, poolID)
	return &adminAuthProvider{
		client:   cip.NewFromConfig(cfg),
		clientID: adminClientID,
		jwks:     jwks.New(issuer + "/.well-known/jwks.json"),
		issuer:   issuer,
	}
}

func (a *adminAuthProvider) SignIn(ctx context.Context, email, password string) (domain.AdminSession, error) {
	out, err := a.client.InitiateAuth(ctx, &cip.InitiateAuthInput{
		ClientId:       aws.String(a.clientID),
		AuthFlow:       types.AuthFlowTypeUserPasswordAuth,
		AuthParameters: map[string]string{"USERNAME": email, "PASSWORD": password},
	})
	if err != nil {
		return domain.AdminSession{}, err
	}
	if out.ChallengeName == types.ChallengeNameTypeNewPasswordRequired {
		return domain.AdminSession{NewPasswordNeeded: true, Session: aws.ToString(out.Session)}, nil
	}
	return adminSession(out.AuthenticationResult)
}

func (a *adminAuthProvider) RespondNewPassword(ctx context.Context, email, session, newPassword string) (domain.AdminSession, error) {
	out, err := a.client.RespondToAuthChallenge(ctx, &cip.RespondToAuthChallengeInput{
		ClientId:           aws.String(a.clientID),
		ChallengeName:      types.ChallengeNameTypeNewPasswordRequired,
		Session:            aws.String(session),
		ChallengeResponses: map[string]string{"USERNAME": email, "NEW_PASSWORD": newPassword},
	})
	if err != nil {
		return domain.AdminSession{}, err
	}
	return adminSession(out.AuthenticationResult)
}

func (a *adminAuthProvider) Refresh(ctx context.Context, refreshToken string) (domain.AdminSession, error) {
	out, err := a.client.InitiateAuth(ctx, &cip.InitiateAuthInput{
		ClientId:       aws.String(a.clientID),
		AuthFlow:       types.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{"REFRESH_TOKEN": refreshToken},
	})
	if err != nil {
		return domain.AdminSession{}, err
	}
	return adminSession(out.AuthenticationResult)
}

func (a *adminAuthProvider) Verify(ctx context.Context, accessToken string) (domain.Admin, []string, error) {
	token, err := jwt.Parse(accessToken, a.jwks.Keyfunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(a.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return domain.Admin{}, nil, err
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	if claims["token_use"] != "access" || claims["client_id"] != a.clientID {
		return domain.Admin{}, nil, errors.New("not an admin access token")
	}
	username, _ := claims["username"].(string)
	var groups []string
	if raw, ok := claims["cognito:groups"].([]any); ok {
		for _, g := range raw {
			if s, ok := g.(string); ok {
				groups = append(groups, s)
			}
		}
	}
	return domain.Admin{Username: username, Email: username}, groups, nil
}

func adminSession(r *types.AuthenticationResultType) (domain.AdminSession, error) {
	if r == nil {
		return domain.AdminSession{}, errors.New("no authentication result")
	}
	return domain.AdminSession{
		AccessToken:  aws.ToString(r.AccessToken),
		RefreshToken: aws.ToString(r.RefreshToken),
		ExpiresIn:    r.ExpiresIn,
	}, nil
}
