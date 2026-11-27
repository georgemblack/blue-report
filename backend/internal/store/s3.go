package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 is a Store backed by an S3 bucket. Cloudflare R2 speaks the same API,
// so this is used for R2 too.
//
// All keys can be put under a prefix (for example "engine/"), so the store
// can share a bucket without touching anything outside its prefix.
type S3 struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3(client *s3.Client, bucket, prefix string) *S3 {
	return &S3{client: client, bucket: bucket, prefix: prefix}
}

func (s *S3) Put(ctx context.Context, key string, body []byte, meta Meta) error {
	in := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.prefix + key),
		Body:   bytes.NewReader(body),
	}
	if meta.ContentType != "" {
		in.ContentType = aws.String(meta.ContentType)
	}
	if meta.CacheControl != "" {
		in.CacheControl = aws.String(meta.CacheControl)
	}
	if _, err := s.client.PutObject(ctx, in); err != nil {
		return fmt.Errorf("store: putting s3://%s/%s%s: %w", s.bucket, s.prefix, key, err)
	}
	return nil
}

// List pages through every matching key (S3 returns at most 1,000 at a time).
func (s *S3) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.prefix + prefix),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("store: listing s3://%s/%s%s: %w", s.bucket, s.prefix, prefix, err)
		}
		for _, obj := range page.Contents {
			keys = append(keys, strings.TrimPrefix(aws.ToString(obj.Key), s.prefix))
		}
	}
	// S3 already lists keys in sorted order.
	return keys, nil
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.prefix + key),
	})
	var noKey *types.NoSuchKey
	if errors.As(err, &noKey) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("store: getting s3://%s/%s%s: %w", s.bucket, s.prefix, key, err)
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("store: reading s3://%s/%s%s: %w", s.bucket, s.prefix, key, err)
	}
	return body, nil
}
