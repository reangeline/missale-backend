package auth

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
)

// Tokens is the Cognito session handed to the app.
type Tokens struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int32  `json:"expiresIn"`
}

// Cognito keeps one pool user per Apple account, named "apple_<sub>" — the
// Apple sub is the only identifier Apple guarantees on every sign-in (email
// comes once, and may be a relay). Sessions use the Hirefy trick: set a fresh
// random password server-side and run ADMIN_USER_PASSWORD_AUTH, so the app
// never sees a password and Cognito still issues and rotates the tokens.
type Cognito struct {
	client   *cip.Client
	poolID   string
	clientID string
	jwks     *JWKS
	issuer   string
}

func NewCognito(cfg aws.Config, region, poolID, clientID string) *Cognito {
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, poolID)
	return &Cognito{
		client:   cip.NewFromConfig(cfg),
		poolID:   poolID,
		clientID: clientID,
		jwks:     NewJWKS(issuer + "/.well-known/jwks.json"),
		issuer:   issuer,
	}
}

func UsernameFor(appleSub string) string { return "apple_" + appleSub }

// SignIn creates the pool user on first sign-in and returns a new session.
// Returns the user's stable ID (Cognito sub).
func (c *Cognito) SignIn(ctx context.Context, apple AppleIdentity) (string, Tokens, error) {
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
		return "", Tokens{}, fmt.Errorf("cognito user: %w", err)
	}

	password, err := randomPassword()
	if err != nil {
		return "", Tokens{}, err
	}
	if _, err := c.client.AdminSetUserPassword(ctx, &cip.AdminSetUserPasswordInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
		Password: aws.String(password), Permanent: true,
	}); err != nil {
		return "", Tokens{}, fmt.Errorf("set password: %w", err)
	}
	out, err := c.client.AdminInitiateAuth(ctx, &cip.AdminInitiateAuthInput{
		UserPoolId: aws.String(c.poolID), ClientId: aws.String(c.clientID),
		AuthFlow:       types.AuthFlowTypeAdminUserPasswordAuth,
		AuthParameters: map[string]string{"USERNAME": username, "PASSWORD": password},
	})
	if err != nil {
		return "", Tokens{}, fmt.Errorf("authenticate: %w", err)
	}
	return userID, tokens(out.AuthenticationResult), nil
}

// Refresh exchanges a refresh token for a new access token.
func (c *Cognito) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	out, err := c.client.AdminInitiateAuth(ctx, &cip.AdminInitiateAuthInput{
		UserPoolId: aws.String(c.poolID), ClientId: aws.String(c.clientID),
		AuthFlow:       types.AuthFlowTypeRefreshTokenAuth,
		AuthParameters: map[string]string{"REFRESH_TOKEN": refreshToken},
	})
	if err != nil {
		return Tokens{}, fmt.Errorf("refresh: %w", err)
	}
	return tokens(out.AuthenticationResult), nil
}

// Delete removes the pool user; its tokens stop verifying at their expiry
// and refresh fails immediately.
func (c *Cognito) Delete(ctx context.Context, username string) error {
	_, err := c.client.AdminDeleteUser(ctx, &cip.AdminDeleteUserInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
	})
	var notFound *types.UserNotFoundException
	if errors.As(err, &notFound) {
		return nil
	}
	return err
}

// Principal is who an access token belongs to.
type Principal struct {
	UserID   string
	Username string
}

// Verify checks a Cognito access token issued to our app client.
func (c *Cognito) Verify(ctx context.Context, accessToken string) (Principal, error) {
	token, err := jwt.Parse(accessToken, c.jwks.Keyfunc(ctx),
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(c.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Principal{}, err
	}
	claims, _ := token.Claims.(jwt.MapClaims)
	if claims["token_use"] != "access" || claims["client_id"] != c.clientID {
		return Principal{}, errors.New("not an access token for this app")
	}
	sub, _ := claims["sub"].(string)
	username, _ := claims["username"].(string)
	if sub == "" || username == "" {
		return Principal{}, errors.New("access token without sub/username")
	}
	return Principal{UserID: sub, Username: username}, nil
}

func (c *Cognito) userID(ctx context.Context, username string) (string, error) {
	out, err := c.client.AdminGetUser(ctx, &cip.AdminGetUserInput{
		UserPoolId: aws.String(c.poolID), Username: aws.String(username),
	})
	if err != nil {
		return "", err
	}
	return attribute(out.UserAttributes, "sub")
}

func (c *Cognito) create(ctx context.Context, username, email string) (string, error) {
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

func tokens(r *types.AuthenticationResultType) Tokens {
	if r == nil {
		return Tokens{}
	}
	return Tokens{
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
