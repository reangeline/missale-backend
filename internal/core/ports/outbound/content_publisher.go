package outbound

import "context"

// ContentPublisher puts published files where the app downloads them (S3
// behind CloudFront). Immutable files are cached for good; the manifest only
// briefly, so a new release reaches the app within minutes.
type ContentPublisher interface {
	Put(ctx context.Context, path string, body []byte, immutable bool) error
}
