package storage

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/georgemblack/blue-report/pkg/util"
)

// PublishLinkSnapshot publishes the snapshot of the site's data to S3.
// Store a 'latest' version, as well as a timestamped version.
func (a AWS) PublishLinkSnapshot(snapshot []byte) error {
	_, err := a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(a.cfg.PublicBucketName),
		Key:          aws.String("data/top-links.json"),
		Body:         bytes.NewReader(snapshot),
		ContentType:  aws.String("application/json"),
		CacheControl: aws.String("public; max-age=600"), // 10 minutes
	})
	if err != nil {
		slog.Error(util.WrapErr("failed to put object to r2", err).Error())
	}

	return nil
}

// PublishSiteSnapshot publishes the snapshot of the site's data to S3.
func (a AWS) PublishSiteSnapshot(snapshot []byte) error {
	_, err := a.r2.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       aws.String(a.cfg.PublicBucketName),
		Key:          aws.String("data/top-sites.json"),
		Body:         bytes.NewReader(snapshot),
		ContentType:  aws.String("application/json"),
		CacheControl: aws.String("public; max-age=600"), // 10 minutes
	})
	if err != nil {
		slog.Error(util.WrapErr("failed to put object to r2", err).Error())
	}

	return nil
}
