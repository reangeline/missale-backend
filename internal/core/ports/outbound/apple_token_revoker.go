package outbound

import "context"

// AppleTokenRevoker asks Apple to revoke the Sign in with Apple grant behind
// a fresh authorization code, when the account is deleted (App Store
// guideline 5.1.1(v)). We never store an Apple token ourselves: the app asks
// for a fresh authorizationCode right before deleting and sends it once.
type AppleTokenRevoker interface {
	// Revoke exchanges the code for Apple's tokens and revokes the refresh
	// token. The returned error never contains the code or a token, so
	// callers can log it as is.
	Revoke(ctx context.Context, authorizationCode string) error
}
