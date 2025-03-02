package config

import (
	"github.com/georgemblack/blue-report/pkg/secrets"
	"github.com/georgemblack/blue-report/pkg/util"
)

type Config struct {
	BlueskyAPIEndpoint       string
	PublicBucketName         string
	ReadEventsBucketName     string
	WriteEventsBucketName    string
	URLMetadataTableName     string // Name of the DynamoDB table used to store titles for each URL
	URLTranslationsTableName string // Name of the DynamoDB table used to store redirects for each URL, i.e. 'https://sho.rt/url' -> 'https://long.url.com/some/path'
	ValkeyAddress            string
	ValkeyTLSEnabled         bool
	NoralizationQueueName    string
	CloudflareDeployHook     string
	CloudflareAPIToken       string
	CloudflareAccountID      string
}

func New() (Config, error) {
	sm, err := secrets.New()
	if err != nil {
		return Config{}, util.WrapErr("failed to create secrets manager", err)
	}

	deployHook, err := sm.GetDeployHook()
	if err != nil {
		return Config{}, util.WrapErr("failed to get deploy hook", err)
	}

	apiToken, err := sm.GetCloudflareAPIToken()
	if err != nil {
		return Config{}, util.WrapErr("failed to get cloudflare api token", err)
	}

	accountID, err := sm.GetCloudflareAccountID()
	if err != nil {
		return Config{}, util.WrapErr("failed to get cloudflare account id", err)
	}

	return Config{
		BlueskyAPIEndpoint:       util.GetEnvStr("BLUESKY_API_ENDPOINT", "https://public.api.bsky.app"),
		PublicBucketName:         util.GetEnvStr("S3_BUCKET_NAME", "blue-report-test"),
		ReadEventsBucketName:     util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-assets"),
		WriteEventsBucketName:    util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-test"),
		URLMetadataTableName:     util.GetEnvStr("DYNAMO_URL_METADATA_TABLE", "blue-report-url-metadata-test"),
		URLTranslationsTableName: util.GetEnvStr("DYNAMO_URL_TRANSLATIONS_TABLE", "blue-report-url-translations-test"),
		ValkeyAddress:            util.GetEnvStr("VALKEY_ADDRESS", "127.0.0.1:6379"),
		ValkeyTLSEnabled:         util.GetEnvBool("VALKEY_TLS_ENABLED", false),
		NoralizationQueueName:    util.GetEnvStr("SQS_NORMALIZATION_QUEUE_NAME", "blue-report-normalization-test"),
		CloudflareDeployHook:     deployHook,
		CloudflareAPIToken:       apiToken,
		CloudflareAccountID:      accountID,
	}, nil
}
