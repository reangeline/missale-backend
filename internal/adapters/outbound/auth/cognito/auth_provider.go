// Package cognito issues and checks sessions with an Amazon Cognito user pool.
package cognito

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

// Cognito keeps one pool user per Apple account, named "apple_<sub>" — the
// Apple sub is the only identifier Apple guarantees on every sign-in (email
// comes once, and may be a relay). Sessions use the Hirefy trick: set a fresh
// random password server-side and run ADMIN_USER_PASSWORD_AUTH, so the app
// never sees a password and Cognito still issues and rotates the tokens.
type authProvider struct {
	client   *cip.Client
	poolID   string
	clientID string
	jwks     *jwks.JWKS
	issuer   string
}

func NewAuthProvider(cfg aws.Config, region, poolID, clientID string) outbound.AuthProvider {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, poolID)
	return &authProvider{
		client:   cip.NewFromConfig(cfg),
		poolID:   poolID,
		clientID: clientID,
		jwks:     jwks.New(issuer + "/.well-known/jwks.json"),
		issuer:   issuer,
	}
}

func UsernameFor(appleSub string) string { return "apple_" + appleSub }

// SignIn creates the pool user on first sign-in and returns a new session.
// Returns the user's stable ID (Cognito sub).
func (c *authProvider) SignIn(ctx context.Context, apple domain.AppleIdentity) (string, domain.Session, error) {
	username := UsernameFor(apple.Sub)
	userID, err := c.userID(ctx, username)
	var notFound *types.UserNotFoundException
	if errors.As(err, &notFound) {
		userID, err = c.create(ctx, username, apple.Email)
		var exists *types.UsernameExistsException
		if errors.As(err, &exists) { // two first sign-ins raced
			userID, err = c.userID(ctx, username)
		}
	}
	if err != nil {
		return "", domain.Session{}, fmt.Errorf("cognito user: %w", err)
	}

	password, err := randomPassword()
	if err != nil {
		return "", domain.Session{}, err
	}
	if _, err := c.client.AdminSetUserPassword(ctx, &cip.AdminSetUserPasswordInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
		Password: aws.String(password), Permanent: true,
	}); err != nil {
		return "", domain.Session{}, fmt.Errorf("set password: %w", err)
	}
	out, err := c.client.AdminInitiateAuth(ctx, &cip.AdminInitiateAuthInput{
		UserPoolId: aws.String(c.poolID), ClientId: aws.String(c.clientID),
		AuthFlow:       types.AuthFlowTypeAdminUserPasswordAuth,
		AuthParameters: map[string]string{"USERNAME": username, "PASSWORD": password},
	})
	if err != nil {
		return "", domain.Session{}, fmt.Errorf("authenticate: %w", err)
	}
	return userID, session(out.AuthenticationResult), nil
}

// Refresh exchanges a refresh token for a new access token.
func (c *authProvider) Refresh(ctx context.Context, refreshToken string) (domain.Session, error) {
	out, err := c.client.AdminInitiateAuth(ctx, &cip.AdminInitiateAuthInput{
		UserPoolId: aws.String(c.poolID), ClientId: aws.String(c.clientID),
		AuthFlow:       types.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{"REFRESH_TOKEN": refreshToken},
	})
	if err != nil {
		return domain.Session{}, fmt.Errorf("refresh: %w", err)
	}
	return session(out.AuthenticationResult), nil
}

// Delete removes the pool user; its tokens stop verifying at their expiry
// and refresh fails immediately.
func (c *authProvider) Delete(ctx context.Context, username string) error {
	_, err := c.client.AdminDeleteUser(ctx, &cip.AdminDeleteUserInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
	})
	var notFound *types.UserNotFoundException
	if errors.As(err, &notFound) {
		return nil
	}
	return err
}

// Verify checks a Cognito access token issued to our app client.
func (c *authProvider) Verify(ctx context.Context, accessToken string) (domain.Principal, error) {
	token, err := jwt.Parse(accessToken, c.jwks.Keyfunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(c.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return domain.Principal{}, err
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	if claims["token_use"] != "access" || claims["client_id"] != c.clientID {
		return domain.Principal{}, errors.New("not an access token for this app")
	}
	sub, _ := claims["sub"].(string)
	username, _ := claims["username"].(string)
	if sub == "" || username == "" {
		return domain.Principal{}, errors.New("access token without sub/username")
	}
	return domain.Principal{UserID: sub, Username: username}, nil
}

func (c *authProvider) userID(ctx context.Context, username string) (string, error) {
	out, err := c.client.AdminGetUser(ctx, &cip.AdminGetUserInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
	})
	if err != nil {
		return "", err
	}
	return attribute(out.UserAttributes, "sub")
}

func (c *authProvider) create(ctx context.Context, username, email string) (string, error) {
	var attrs []types.AttributeType
	if email != "" {
		attrs = append(attrs,
			types.AttributeType{Name: aws.String("email"), Value: aws.String(email)},
			types.AttributeType{Name: aws.String("email_verified"), Value: aws.String("true")})
	}
	out, err := c.client.AdminCreateUser(ctx, &cip.AdminCreateUserInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
		MessageAction: types.MessageActionTypeSuppress, UserAttributes: attrs,
	})
	if err != nil {
		return "", err
	}
	return attribute(out.User.Attributes, "sub")
}

func attribute(attrs []types.AttributeType, name string) (string, error) {
	for _, a := range attrs {
		if aws.ToString(a.Name) == name {
			return aws.ToString(a.Value), nil
		}
	}
	return "", fmt.Errorf("attribute %s missing", name)
}

func session(r *types.AuthenticationResultType) domain.Session {
	if r == nil {
		return domain.Session{}
	}
	return domain.Session{
		AccessToken:  aws.ToString(r.AccessToken),
		RefreshToken: aws.ToString(r.RefreshToken),
		ExpiresIn:    r.ExpiresIn,
	}
}

// randomPassword satisfies any pool policy; it is never stored or shown.
func randomPassword() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "Aa1!" + base64.RawURLEncoding.EncodeToString(b), nil
}
