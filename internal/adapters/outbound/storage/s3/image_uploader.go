package s3

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type imageUploader struct {
	presign *awss3.PresignClient
	bucket  string
}

func NewImageUploader(cfg aws.Config, bucket string) outbound.ImageUploader {
	return &imageUploader{presign: awss3.NewPresignClient(awss3.NewFromConfig(cfg)), bucket: bucket}
}

// immutableCache: every upload gets a new random name, so it never changes.
const immutableCache = "public, max-age=31536000, immutable"

// PrepareUpload signs a POST policy for exactly this key, this content type
// and at most maxBytes, valid for ten minutes.
func (u *imageUploader) PrepareUpload(ctx context.Context, key, contentType string, maxBytes int64) (outbound.ImageUpload, error) {
	req, err := u.presign.PresignPostObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	}, func(o *awss3.PresignPostOptions) {
		o.Expires = 10 * time.Minute
		o.Conditions = []any{
			[]any{"content-length-range", 1, maxBytes},
			map[string]string{"Content-Type": contentType},
			map[string]string{"Cache-Control": immutableCache},
		}
	})
	if err != nil {
		return outbound.ImageUpload{}, err
	}
	fields := map[string]string{}
	for k, v := range req.Values {
		fields[k] = v
	}
	fields["Content-Type"] = contentType
	fields["Cache-Control"] = immutableCache
	return outbound.ImageUpload{URL: req.URL, Fields: fields, MaxBytes: maxBytes}, nil
}
