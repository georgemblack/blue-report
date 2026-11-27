package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/georgemblack/blue-report/backend/internal/browserrun"
	"github.com/georgemblack/blue-report/backend/internal/config"
	"github.com/georgemblack/blue-report/backend/internal/feed"
	"github.com/georgemblack/blue-report/backend/internal/hydrate"
	"github.com/georgemblack/blue-report/backend/internal/resolve"
	"github.com/georgemblack/blue-report/backend/internal/store"
)

// awsConfig loads AWS settings. In ECS, credentials come from the task role
// and the region from the environment. us-west-2 is only a fallback for
// running elsewhere.
func awsConfig(ctx context.Context) (aws.Config, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, fmt.Errorf("loading aws config: %w", err)
	}
	if awsCfg.Region == "" {
		awsCfg.Region = "us-west-2"
	}
	return awsCfg, nil
}

// eventStore returns where event files are kept: the S3 bucket if one is
// configured, otherwise a local folder. It also says which, for logging.
func eventStore(awsCfg aws.Config, cfg config.Config) (store.Store, string) {
	if cfg.EventsBucket == "" {
		return store.NewLocal(cfg.DataDir), "folder " + cfg.DataDir
	}
	return store.NewS3(s3.NewFromConfig(awsCfg), cfg.EventsBucket, ""), "s3://" + cfg.EventsBucket
}

// publishStores returns where public files go: the R2 bucket if one is
// configured, otherwise a local folder. The first store is for the engine's
// own files (under PUBLISH_PREFIX); the second is the top of the bucket,
// where thumbnails shared with v1 live.
func publishStores(ctx context.Context, cfg config.Config) (published, thumbnails store.Store, where string, err error) {
	if cfg.PublishBucket == "" {
		local := store.NewLocal(cfg.PublishDir)
		return local, local, "folder " + cfg.PublishDir, nil
	}
	// R2 speaks the S3 API, at an address that includes the Cloudflare
	// account ID, with its own access keys and the region "auto".
	r2Cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, "")),
	)
	if err != nil {
		return nil, nil, "", fmt.Errorf("loading r2 config: %w", err)
	}
	client := s3.NewFromConfig(r2Cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID))
		// Only send and check checksums where the API requires them, since
		// R2 doesn't support every checksum the SDK would add by default.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	published = store.NewS3(client, cfg.PublishBucket, cfg.PublishPrefix)
	thumbnails = store.NewS3(client, cfg.PublishBucket, "")
	return published, thumbnails, fmt.Sprintf("r2://%s/%s", cfg.PublishBucket, cfg.PublishPrefix), nil
}

// titleStore returns where link titles are kept: v1's DynamoDB table if one
// is configured, otherwise memory.
func titleStore(awsCfg aws.Config, cfg config.Config) (hydrate.Titles, string) {
	if cfg.MetadataTable == "" {
		return hydrate.NewMemoryTitles(), "memory"
	}
	return hydrate.NewDynamoTitles(dynamodb.NewFromConfig(awsCfg), cfg.MetadataTable), "dynamodb:" + cfg.MetadataTable
}

// redirectStore returns where the redirect resolver saves what it finds:
// DynamoDB if a table is configured, otherwise memory.
func redirectStore(awsCfg aws.Config, cfg config.Config) (resolve.Store, string) {
	if cfg.RedirectsTable == "" {
		return resolve.NewMemoryStore(), "memory"
	}
	return resolve.NewDynamoStore(dynamodb.NewFromConfig(awsCfg), cfg.RedirectsTable), "dynamodb:" + cfg.RedirectsTable
}

// feedStore returns where feed entries are kept: DynamoDB if a table is
// configured, otherwise memory.
func feedStore(awsCfg aws.Config, cfg config.Config) (feed.Store, string) {
	if cfg.FeedTable == "" {
		return feed.NewMemoryStore(), "memory"
	}
	return feed.NewDynamoStore(dynamodb.NewFromConfig(awsCfg), cfg.FeedTable), "dynamodb:" + cfg.FeedTable
}

// browser returns a Browser Run client if Cloudflare credentials are
// configured, otherwise nil (CardyB only).
func browser(cfg config.Config) (hydrate.Browser, bool) {
	if cfg.CloudflareAccountID == "" || cfg.CloudflareAPIToken == "" {
		return nil, false
	}
	return browserrun.New(cfg.CloudflareAccountID, cfg.CloudflareAPIToken), true
}
