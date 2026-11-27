package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/georgemblack/blue-report/backend/internal/config"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// eventStore returns where event files are kept: the S3 bucket if one is
// configured, otherwise a local folder. It also says which, for logging.
func eventStore(ctx context.Context, cfg config.Config) (store.Store, string, error) {
	if cfg.EventsBucket == "" {
		return store.NewLocal(cfg.DataDir), "folder " + cfg.DataDir, nil
	}
	// In ECS, credentials come from the task role and the region from the
	// environment. us-west-2 is only a fallback for running elsewhere.
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("loading aws config: %w", err)
	}
	if awsCfg.Region == "" {
		awsCfg.Region = "us-west-2"
	}
	return store.NewS3(s3.NewFromConfig(awsCfg), cfg.EventsBucket, ""), "s3://" + cfg.EventsBucket, nil
}

// publishStore returns where public files go: the R2 bucket if one is
// configured, otherwise a local folder.
func publishStore(ctx context.Context, cfg config.Config) (store.Store, string, error) {
	if cfg.PublishBucket == "" {
		return store.NewLocal(cfg.PublishDir), "folder " + cfg.PublishDir, nil
	}
	// R2 speaks the S3 API, at an address that includes the Cloudflare
	// account ID, with its own access keys and the region "auto".
	r2Cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, "")),
	)
	if err != nil {
		return nil, "", fmt.Errorf("loading r2 config: %w", err)
	}
	client := s3.NewFromConfig(r2Cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID))
		// Only send and check checksums where the API requires them, since
		// R2 doesn't support every checksum the SDK would add by default.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return store.NewS3(client, cfg.PublishBucket, cfg.PublishPrefix), fmt.Sprintf("r2://%s/%s", cfg.PublishBucket, cfg.PublishPrefix), nil
}
