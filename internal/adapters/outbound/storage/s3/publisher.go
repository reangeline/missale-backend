// Package s3 publishes the app's content files to the bucket CloudFront serves.
package s3

import (
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/reangeline/missale-backend/internal/core/ports/outbound"
)

type publisher struct {
	client *awss3.Client
	bucket string
}

func NewPublisher(cfg aws.Config, bucket string) outbound.ContentPublisher {
	return &publisher{client: awss3.NewFromConfig(cfg), bucket: bucket}
}

func (p *publisher) Put(ctx context.Context, path string, body []byte, immutable bool) error {
	cache := "public, max-age=60"
	if immutable {
		cache = "public, max-age=31536000, immutable"
	}
	_, err := p.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket:       aws.String(p.bucket),
		Key:          aws.String(path),
		Body:         bytes.NewReader(body),
		ContentType:  aws.String("application/json; charset=utf-8"),
		CacheControl: aws.String(cache),
	})
	return err
}
