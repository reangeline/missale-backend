package domain

// User is an account, created on the first Sign in with Apple.
// ID is the Cognito sub; AppleSub is Apple's stable user identifier.
type User struct {
	ID       string
	AppleSub string
}

// AppleIdentity is what a verified Sign in with Apple identity token says.
// Email comes only on the first sign-in and may be a private relay address.
type AppleIdentity struct {
	Sub   string
	Email string
}

// Principal is who an access token belongs to.
type Principal struct {
	UserID   string
	Username string
}

// Session is what the app keeps in the Keychain.
type Session struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int32  `json:"expiresIn"`
}
