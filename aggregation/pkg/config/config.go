package config

import (
	"encoding/json"
	"log/slog"

	"github.com/georgemblack/blue-report/pkg/secrets"
	"github.com/georgemblack/blue-report/pkg/util"
)

type Config struct {
	BlueskyAPIEndpoint          string
	PublicBucketName            string
	ReadEventsBucketName        string
	WriteEventsBucketName       string
	URLMetadataTableName        string // Name of the DynamoDB table used to store titles for each URL
	URLTranslationsTableName    string // Name of the DynamoDB table used to store redirects for each URL, i.e. 'https://sho.rt/url' -> 'https://long.url.com/some/path'
	FeedTableName               string // Name of the DynamoDB table used to store items that get posted by the bot, and to the RSS feed
	ValkeyAddress               string
	ValkeyTLSEnabled            bool
	NoralizationQueueName       string
	CloudflareAccountID         string
	CloudflareDeployHook        string
	CloudflareAPIToken          string
	CloudflareR2AccessKeyID     string
	CloudflareR2SecretAccessKey string
}

func New() (Config, error) {
	sm, err := secrets.New()
	if err != nil {
		return Config{}, util.WrapErr("failed to create secrets manager", err)
	}

	accountID, err := sm.GetCloudflareAccountID()
	if err != nil {
		return Config{}, util.WrapErr("failed to get cloudflare account id", err)
	}

	deployHook, err := sm.GetDeployHook()
	if err != nil {
		return Config{}, util.WrapErr("failed to get deploy hook", err)
	}

	apiToken, err := sm.GetCloudflareAPIToken()
	if err != nil {
		return Config{}, util.WrapErr("failed to get cloudflare api token", err)
	}

	r2AccessKeyID, err := sm.GetCloudflareR2AccessKeyID()
	if err != nil {
		return Config{}, util.WrapErr("failed to get cloudflare r2 access key id", err)
	}

	r2SecretAccessKey, err := sm.GetCloudflareR2SecretAccessKey()
	if err != nil {
		return Config{}, util.WrapErr("failed to get cloudflare r2 secret access key", err)
	}

	result := Config{
		BlueskyAPIEndpoint:          util.GetEnvStr("BLUESKY_API_ENDPOINT", "https://public.api.bsky.app"),
		PublicBucketName:            util.GetEnvStr("S3_BUCKET_NAME", "blue-report-test"),
		ReadEventsBucketName:        util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-assets"),
		WriteEventsBucketName:       util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-test"),
		URLMetadataTableName:        util.GetEnvStr("DYNAMO_URL_METADATA_TABLE", "blue-report-url-metadata-test"),
		URLTranslationsTableName:    util.GetEnvStr("DYNAMO_URL_TRANSLATIONS_TABLE", "blue-report-url-translations-test"),
		FeedTableName:               util.GetEnvStr("DYNAMO_FEED_TABLE", "blue-report-feed-test"),
		ValkeyAddress:               util.GetEnvStr("VALKEY_ADDRESS", "127.0.0.1:6379"),
		ValkeyTLSEnabled:            util.GetEnvBool("VALKEY_TLS_ENABLED", false),
		NoralizationQueueName:       util.GetEnvStr("SQS_NORMALIZATION_QUEUE_NAME", "blue-report-normalization-test"),
		CloudflareAccountID:         accountID,
		CloudflareDeployHook:        deployHook,
		CloudflareAPIToken:          apiToken,
		CloudflareR2AccessKeyID:     r2AccessKeyID,
		CloudflareR2SecretAccessKey: r2SecretAccessKey,
	}

	// Marshal to JSON and print if debug is enabled
	data, err := json.Marshal(result)
	if err != nil {
		slog.Warn(util.WrapErr("failed to marshal config", err).Error())
	}
	slog.Debug("generated config", "config", string(data))

	return result, nil
}
