package config

import (
	"github.com/georgemblack/blue-report/pkg/secrets"
	"github.com/georgemblack/blue-report/pkg/util"
)

type Config struct {
	BlueskyAPIEndpoint    string
	PublicBucketName      string
	ReadEventsBucketName  string
	WriteEventsBucketName string
	URLMetadataTableName  string
	ValkeyAddress         string
	ValkeyTLSEnabled      bool
	DeployHookURL         string
}

func New() (Config, error) {
	sm, err := secrets.New()
	if err != nil {
		return Config{}, util.WrapErr("failed to create secrets manager", err)
	}

	deployHookURL, err := sm.GetDeployHook()
	if err != nil {
		return Config{}, util.WrapErr("failed to get deploy hook", err)
	}

	return Config{
		BlueskyAPIEndpoint:    util.GetEnvStr("BLUESKY_API_ENDPOINT", "https://bluesky-web.org"),
		PublicBucketName:      util.GetEnvStr("S3_BUCKET_NAME", "blue-report-test"),
		ReadEventsBucketName:  util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-assets"),
		WriteEventsBucketName: util.GetEnvStr("S3_ASSETS_BUCKET_NAME", "blue-report-test"),
		URLMetadataTableName:  util.GetEnvStr("DYNAMO_URL_METADATA_TABLE", "blue-report-url-metadata-test"),
		ValkeyAddress:         util.GetEnvStr("VALKEY_ADDRESS", "127.0.0.1:6379"),
		ValkeyTLSEnabled:      util.GetEnvBool("VALKEY_TLS_ENABLED", false),
		DeployHookURL:         deployHookURL,
	}, nil
}
