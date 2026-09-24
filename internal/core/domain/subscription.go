package domain

// SubscriptionTransaction is the part of a StoreKit 2 transaction the server checks.
type SubscriptionTransaction struct {
	BundleID              string `json:"bundleId"`
	ProductID             string `json:"productId"`
	OriginalTransactionID string `json:"originalTransactionId"`
	ExpiresDate           int64  `json:"expiresDate"`
	RevocationDate        int64  `json:"revocationDate"`
	Environment           string `json:"environment"`
}
